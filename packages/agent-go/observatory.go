// path: packages/agent-go/observatory.go
package observatory

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Config configures the Observatory Go Agent.
type Config struct {
	Service         string
	Endpoint        string
	APIKey          string
	SampleRate      float64
	FlushIntervalMs int
	BatchSize       int
}

// Span represents an application telemetry span.
type Span struct {
	StartTs    time.Time              `json:"start_ts"`
	TraceID    string                 `json:"trace_id"`
	SpanID     string                 `json:"span_id"`
	ParentID   string                 `json:"parent_id,omitempty"`
	Service    string                 `json:"service"`
	Name       string                 `json:"name"`
	Kind       string                 `json:"kind"`
	DurationNs int64                  `json:"duration_ns"`
	Status     string                 `json:"status"`
	Attrs      map[string]interface{} `json:"attrs"`

	client    *Client
	startTime time.Time
}

type traceCtxKey struct{}

// TraceContext holds the active W3C trace identifiers.
type TraceContext struct {
	TraceID string
	SpanID  string
}

// FromContext extracts the active TraceContext if present.
func FromContext(ctx context.Context) (TraceContext, bool) {
	if ctx == nil {
		return TraceContext{}, false
	}
	tc, ok := ctx.Value(traceCtxKey{}).(TraceContext)
	return tc, ok
}

// WithContext stores a TraceContext in the context.
func WithContext(ctx context.Context, tc TraceContext) context.Context {
	return context.WithValue(ctx, traceCtxKey{}, tc)
}

// Client represents an active Observatory APM telemetry client.
type Client struct {
	cfg        Config
	spanQueue  chan Span
	stopSignal chan struct{}
	wg         sync.WaitGroup
	httpClient *http.Client
}

var (
	defaultClient *Client
	defaultMu     sync.RWMutex
)

// Init initializes the global Observatory agent client.
func Init(cfg Config) (*Client, error) {
	if cfg.Service == "" {
		return nil, errors.New("observatory.Init: Service name is required")
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = os.Getenv("OBSERVATORY_ENDPOINT")
		if cfg.Endpoint == "" {
			cfg.Endpoint = "http://localhost:8080"
		}
	}
	cfg.Endpoint = strings.TrimRight(cfg.Endpoint, "/")

	if cfg.SampleRate <= 0 || cfg.SampleRate > 1.0 {
		cfg.SampleRate = 0.10 // Default to 10% head-based ratio sampling
	}
	if cfg.FlushIntervalMs <= 0 {
		cfg.FlushIntervalMs = 500
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}

	c := &Client{
		cfg:        cfg,
		spanQueue:  make(chan Span, 10000),
		stopSignal: make(chan struct{}),
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 50,
			},
		},
	}

	c.wg.Add(1)
	go c.worker()

	defaultMu.Lock()
	defaultClient = c
	defaultMu.Unlock()

	return c, nil
}

// DefaultClient returns the currently initialized singleton client.
func DefaultClient() *Client {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultClient
}

// StartSpan starts a new span, linking to any active parent in ctx.
func (c *Client) StartSpan(ctx context.Context, name string) (context.Context, *Span) {
	now := time.Now().UTC()
	var traceID string
	var parentSpanID string

	if parent, ok := FromContext(ctx); ok {
		traceID = parent.TraceID
		parentSpanID = parent.SpanID
	} else {
		traceID = randomHex(16)
	}

	spanID := randomHex(8)
	kind := "SPAN_KIND_SERVER"
	if parentSpanID != "" {
		kind = "SPAN_KIND_INTERNAL"
	}

	s := &Span{
		StartTs:    now,
		TraceID:    traceID,
		SpanID:     spanID,
		ParentID:   parentSpanID,
		Service:    c.cfg.Service,
		Name:       name,
		Kind:       kind,
		Status:     "STATUS_CODE_OK",
		Attrs:      make(map[string]interface{}),
		client:     c,
		startTime:  now,
	}

	s.Attrs["telemetry.sdk.language"] = "go"
	s.Attrs["telemetry.sdk.name"] = "observatory-agent-go"

	newCtx := WithContext(ctx, TraceContext{TraceID: traceID, SpanID: spanID})
	return newCtx, s
}

// StartSpan starts a span using the global singleton client.
func StartSpan(ctx context.Context, name string) (context.Context, *Span) {
	c := DefaultClient()
	if c == nil {
		traceID := randomHex(16)
		spanID := randomHex(8)
		newCtx := WithContext(ctx, TraceContext{TraceID: traceID, SpanID: spanID})
		return newCtx, &Span{
			TraceID: traceID,
			SpanID:  spanID,
			Name:    name,
		}
	}
	return c.StartSpan(ctx, name)
}

// SetStatus records an error or ok status.
func (s *Span) SetStatus(status string) {
	s.Status = status
}

// SetAttr sets a custom span attribute.
func (s *Span) SetAttr(key string, val interface{}) {
	if s.Attrs == nil {
		s.Attrs = make(map[string]interface{})
	}
	s.Attrs[key] = val
}

// shouldSample evaluates deterministic 64-bit ratio sampling and error retention.
func (c *Client) shouldSample(span *Span) bool {
	if span.Status == "STATUS_CODE_ERROR" {
		return true // Always sample errors
	}
	if c.cfg.SampleRate >= 1.0 {
		return true
	}
	if c.cfg.SampleRate <= 0.0 {
		return false
	}

	clean := strings.TrimPrefix(span.TraceID, "0x")
	clean = strings.ReplaceAll(clean, "-", "")
	if len(clean) >= 16 {
		last16 := clean[len(clean)-16:]
		b, err := hex.DecodeString(last16)
		if err == nil && len(b) == 8 {
			val := binary.BigEndian.Uint64(b)
			threshold := uint64(c.cfg.SampleRate * float64(math.MaxUint64))
			return val < threshold
		}
	}
	return true
}

// End finishes the span and schedules it for batched export.
func (s *Span) End() {
	if s.client == nil {
		return
	}
	s.DurationNs = time.Since(s.startTime).Nanoseconds()

	if !s.client.shouldSample(s) {
		return
	}

	select {
	case s.client.spanQueue <- *s:
	default:
		// Queue full: drop span to protect caller
	}
}

// Middleware returns an HTTP handler middleware for automatic incoming request instrumentation.
func (c *Client) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceParent := r.Header.Get("traceparent")
		var traceID, parentSpanID string

		if strings.HasPrefix(traceParent, "00-") {
			parts := strings.Split(traceParent, "-")
			if len(parts) >= 3 && len(parts[1]) == 32 {
				traceID = parts[1]
				parentSpanID = parts[2]
			}
		}

		if traceID == "" {
			traceID = randomHex(16)
		}
		spanID := randomHex(8)

		ctx := WithContext(r.Context(), TraceContext{TraceID: traceID, SpanID: spanID})
		start := time.Now().UTC()

		w.Header().Set("traceparent", fmt.Sprintf("00-%s-%s-01", traceID, spanID))

		rw := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r.WithContext(ctx))

		status := "STATUS_CODE_OK"
		if rw.statusCode >= 500 {
			status = "STATUS_CODE_ERROR"
		}

		span := Span{
			StartTs:    start,
			TraceID:    traceID,
			SpanID:     spanID,
			ParentID:   parentSpanID,
			Service:    c.cfg.Service,
			Name:       r.URL.Path,
			Kind:       "SPAN_KIND_SERVER",
			DurationNs: time.Since(start).Nanoseconds(),
			Status:     status,
			Attrs: map[string]interface{}{
				"http.method":      r.Method,
				"http.status_code": rw.statusCode,
				"http.url":         r.URL.String(),
			},
			client: c,
		}

		if c.shouldSample(&span) {
			select {
			case c.spanQueue <- span:
			default:
			}
		}
	})
}

// worker loops draining the span queue and flushing in batches.
func (c *Client) worker() {
	defer c.wg.Done()
	ticker := time.NewTicker(time.Duration(c.cfg.FlushIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	batch := make([]Span, 0, c.cfg.BatchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		c.sendSpans(batch)
		batch = make([]Span, 0, c.cfg.BatchSize)
	}

	for {
		select {
		case <-c.stopSignal:
			flush()
			return
		case span := <-c.spanQueue:
			batch = append(batch, span)
			if len(batch) >= c.cfg.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (c *Client) sendSpans(spans []Span) {
	data, err := json.Marshal(spans)
	if err != nil {
		return
	}

	url := c.cfg.Endpoint + "/v1/traces"
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("X-API-Key", c.cfg.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

// Close flushes all queued spans and stops background workers.
func (c *Client) Close() {
	close(c.stopSignal)
	c.wg.Wait()
}

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *statusResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func randomHex(bytesLen int) string {
	b := make([]byte, bytesLen)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

