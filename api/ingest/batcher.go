// path: api/ingest/batcher.go
package ingest

import (
	"context"
	"sync"
	"time"
)

// FlushFunc defines the signature for batch commit operations.
type FlushFunc[T any] func(ctx context.Context, batch []T) error

// Batcher provides a thread-safe, bounded-queue batch processor.
// Items are buffered in an in-memory channel. When the batch size reaches
// MaxBatchSize or the FlushInterval ticker elapses, the batch is committed.
type Batcher[T any] struct {
	telemetryType string
	queue         chan T
	maxBatchSize  int
	flushInterval time.Duration
	flushFn       FlushFunc[T]

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewBatcher initializes and starts the background worker for a bounded batcher.
func NewBatcher[T any](
	telemetryType string,
	queueCapacity int,
	maxBatchSize int,
	flushInterval time.Duration,
	flushFn FlushFunc[T],
) *Batcher[T] {
	if queueCapacity <= 0 {
		queueCapacity = 10000
	}
	if maxBatchSize <= 0 {
		maxBatchSize = 1000
	}
	if flushInterval <= 0 {
		flushInterval = 500 * time.Millisecond
	}

	ctx, cancel := context.WithCancel(context.Background())
	b := &Batcher[T]{
		telemetryType: telemetryType,
		queue:         make(chan T, queueCapacity),
		maxBatchSize:  maxBatchSize,
		flushInterval: flushInterval,
		flushFn:       flushFn,
		ctx:           ctx,
		cancel:        cancel,
	}

	b.wg.Add(1)
	go b.worker()

	return b
}

// Enqueue attempts to push an item into the bounded queue.
// If the buffer is full, it returns false immediately (backpressure trigger).
func (b *Batcher[T]) Enqueue(item T) bool {
	select {
	case b.queue <- item:
		IngestQueueDepth.WithLabelValues(b.telemetryType).Set(float64(len(b.queue)))
		return true
	default:
		// Queue full: apply backpressure immediately
		IngestReceivedTotal.WithLabelValues(b.telemetryType, "rejected").Inc()
		return false
	}
}

// worker coordinates batch aggregation and timed flushes.
func (b *Batcher[T]) worker() {
	defer b.wg.Done()
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	batch := make([]T, 0, b.maxBatchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		start := time.Now()
		_ = b.flushFn(b.ctx, batch)
		duration := time.Since(start).Seconds()

		IngestBatchFlushDuration.WithLabelValues(b.telemetryType).Observe(duration)
		IngestBatchSize.WithLabelValues(b.telemetryType).Observe(float64(len(batch)))
		IngestPointsTotal.WithLabelValues(b.telemetryType).Add(float64(len(batch)))

		batch = make([]T, 0, b.maxBatchSize)
		IngestQueueDepth.WithLabelValues(b.telemetryType).Set(float64(len(b.queue)))
	}

	for {
		select {
		case <-b.ctx.Done():
			// Drain remaining queue items on shutdown
			for {
				select {
				case item := <-b.queue:
					batch = append(batch, item)
					if len(batch) >= b.maxBatchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}

		case item := <-b.queue:
			batch = append(batch, item)
			if len(batch) >= b.maxBatchSize {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

// Stop flushes remaining items and waits for the worker loop to terminate.
func (b *Batcher[T]) Stop() {
	b.cancel()
	b.wg.Wait()
}

