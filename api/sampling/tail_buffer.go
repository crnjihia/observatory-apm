// path: api/sampling/tail_buffer.go
package sampling

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"observatory-apm/api/ingest"

	"github.com/redis/go-redis/v9"
)

// TraceBuffer manages temporary span buffering across an active decision window.
type TraceBuffer interface {
	// AddSpan appends a span for the trace and marks its arrival timestamp.
	AddSpan(ctx context.Context, span ingest.Span) error
	// GetSpans returns all buffered spans associated with traceID.
	GetSpans(ctx context.Context, traceID string) ([]ingest.Span, error)
	// Delete removes all spans and active metadata for traceID.
	Delete(ctx context.Context, traceID string) error
	// GetExpiredTraces returns all trace IDs whose initial span arrived prior to cutoff.
	GetExpiredTraces(ctx context.Context, cutoff time.Time) ([]string, error)
	// ActiveTraceCount returns the number of traces currently residing in the buffer.
	ActiveTraceCount(ctx context.Context) (int, error)
	// Close releases any persistent client connections or background memory.
	Close() error
}

// ============================================================================
// In-Memory Trace Buffer (zero external dependency fallback & test engine)
// ============================================================================

type memTraceRecord struct {
	firstSeen time.Time
	lastSeen  time.Time
	spans     []ingest.Span
}

// MemoryTraceBuffer implements TraceBuffer entirely in-process using sync.RWMutex.
type MemoryTraceBuffer struct {
	mu     sync.RWMutex
	traces map[string]*memTraceRecord
}

// NewMemoryTraceBuffer constructs an initialized MemoryTraceBuffer.
func NewMemoryTraceBuffer() *MemoryTraceBuffer {
	return &MemoryTraceBuffer{
		traces: make(map[string]*memTraceRecord),
	}
}

// AddSpan appends a span to in-memory storage.
func (m *MemoryTraceBuffer) AddSpan(_ context.Context, span ingest.Span) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	rec, exists := m.traces[span.TraceID]
	if !exists {
		rec = &memTraceRecord{
			firstSeen: now,
			lastSeen:  now,
			spans:     make([]ingest.Span, 0, 4),
		}
		m.traces[span.TraceID] = rec
	} else {
		rec.lastSeen = now
	}

	rec.spans = append(rec.spans, span)
	return nil
}

// GetSpans retrieves all spans recorded for traceID.
func (m *MemoryTraceBuffer) GetSpans(_ context.Context, traceID string) ([]ingest.Span, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rec, exists := m.traces[traceID]
	if !exists {
		return nil, nil
	}

	copied := make([]ingest.Span, len(rec.spans))
	copy(copied, rec.spans)
	return copied, nil
}

// Delete purges trace data from the memory buffer.
func (m *MemoryTraceBuffer) Delete(_ context.Context, traceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.traces, traceID)
	return nil
}

// GetExpiredTraces returns all trace IDs whose first span was seen before cutoff.
func (m *MemoryTraceBuffer) GetExpiredTraces(_ context.Context, cutoff time.Time) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var expired []string
	for traceID, rec := range m.traces {
		if rec.firstSeen.Before(cutoff) {
			expired = append(expired, traceID)
		}
	}
	return expired, nil
}

// ActiveTraceCount returns the current count of buffered traces.
func (m *MemoryTraceBuffer) ActiveTraceCount(_ context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.traces), nil
}

// Close implements TraceBuffer.
func (m *MemoryTraceBuffer) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.traces = make(map[string]*memTraceRecord)
	return nil
}

// ============================================================================
// Redis Trace Buffer (distributed ring buffer across multi-instance API)
// ============================================================================

// RedisTraceBuffer implements TraceBuffer backed by Redis 7 lists and sorted sets.
type RedisTraceBuffer struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

// NewRedisTraceBuffer constructs a Redis-backed trace buffer.
func NewRedisTraceBuffer(client *redis.Client) *RedisTraceBuffer {
	return &RedisTraceBuffer{
		client: client,
		prefix: "obs:",
		ttl:    10 * time.Minute,
	}
}

// AddSpan serializes and appends the span to a Redis list and marks arrival in a sorted set.
func (r *RedisTraceBuffer) AddSpan(ctx context.Context, span ingest.Span) error {
	data, err := json.Marshal(span)
	if err != nil {
		return fmt.Errorf("redis_trace_buffer marshal span error: %w", err)
	}

	traceKey := r.prefix + "spans:" + span.TraceID
	activeKey := r.prefix + "active_traces"
	firstSeenKey := r.prefix + "first_seen:" + span.TraceID

	pipe := r.client.Pipeline()
	pipe.RPush(ctx, traceKey, data)
	pipe.Expire(ctx, traceKey, r.ttl)

	// Set NX (only if not exists) the firstSeen timestamp in milliseconds
	nowUnixNano := time.Now().UnixNano()
	pipe.SetNX(ctx, firstSeenKey, nowUnixNano, r.ttl)
	pipe.ZAddNX(ctx, activeKey, redis.Z{
		Score:  float64(nowUnixNano),
		Member: span.TraceID,
	})

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis_trace_buffer pipeline error: %w", err)
	}
	return nil
}

// GetSpans pulls and decodes all serialized spans for traceID from Redis.
func (r *RedisTraceBuffer) GetSpans(ctx context.Context, traceID string) ([]ingest.Span, error) {
	traceKey := r.prefix + "spans:" + traceID
	items, err := r.client.LRange(ctx, traceKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("redis_trace_buffer lrange error: %w", err)
	}

	spans := make([]ingest.Span, 0, len(items))
	for _, raw := range items {
		var s ingest.Span
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			continue
		}
		spans = append(spans, s)
	}
	return spans, nil
}

// Delete cleans up Redis lists and sorted set entries for traceID.
func (r *RedisTraceBuffer) Delete(ctx context.Context, traceID string) error {
	traceKey := r.prefix + "spans:" + traceID
	activeKey := r.prefix + "active_traces"
	firstSeenKey := r.prefix + "first_seen:" + traceID

	pipe := r.client.Pipeline()
	pipe.Del(ctx, traceKey)
	pipe.Del(ctx, firstSeenKey)
	pipe.ZRem(ctx, activeKey, traceID)
	_, err := pipe.Exec(ctx)
	return err
}

// GetExpiredTraces queries the active_traces sorted set for timestamps <= cutoff.UnixNano().
func (r *RedisTraceBuffer) GetExpiredTraces(ctx context.Context, cutoff time.Time) ([]string, error) {
	activeKey := r.prefix + "active_traces"
	maxScore := fmt.Sprintf("%d", cutoff.UnixNano())

	traceIDs, err := r.client.ZRangeByScore(ctx, activeKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: maxScore,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("redis_trace_buffer zrangebyscore error: %w", err)
	}
	return traceIDs, nil
}

// ActiveTraceCount returns the number of items in active_traces sorted set.
func (r *RedisTraceBuffer) ActiveTraceCount(ctx context.Context) (int, error) {
	activeKey := r.prefix + "active_traces"
	count, err := r.client.ZCard(ctx, activeKey).Result()
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// Close releases the Redis client.
func (r *RedisTraceBuffer) Close() error {
	return r.client.Close()
}

// NewTraceBuffer creates a Redis-backed buffer if redisURL is reachable,
// otherwise falling back seamlessly to an in-memory ring buffer.
func NewTraceBuffer(redisURL string) (TraceBuffer, error) {
	if redisURL == "" {
		log.Println("[Sampling] REDIS_URL not configured. Operating in high-performance in-memory buffer mode.")
		return NewMemoryTraceBuffer(), nil
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("[Sampling] Failed to parse REDIS_URL (%s): %v. Falling back to in-memory buffer.", redisURL, err)
		return NewMemoryTraceBuffer(), nil
	}

	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("[Sampling] Redis ping failed (%v). Falling back to in-memory buffer mode.", err)
		_ = client.Close()
		return NewMemoryTraceBuffer(), nil
	}

	log.Printf("[Sampling] Connected to Redis cluster buffer at %s", opt.Addr)
	return NewRedisTraceBuffer(client), nil
}

