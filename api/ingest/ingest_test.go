// path: api/ingest/ingest_test.go
package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestValidateSpan(t *testing.T) {
	valid := &Span{
		TraceID:    "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:     "00f067aa0ba902b7",
		Service:    "mpesa-callback-service",
		Name:       "/v1/callbacks/mpesa",
		StartTs:    time.Now().UTC(),
		DurationNs: 150000000,
	}
	if err := ValidateSpan(valid); err != nil {
		t.Fatalf("expected valid span, got err: %v", err)
	}

	// Missing trace id
	invalid := *valid
	invalid.TraceID = ""
	if err := ValidateSpan(&invalid); err != ErrEmptyTraceID {
		t.Fatalf("expected ErrEmptyTraceID, got: %v", err)
	}

	// Negative duration
	invalid = *valid
	invalid.DurationNs = -5
	if err := ValidateSpan(&invalid); err != ErrNegativeDuration {
		t.Fatalf("expected ErrNegativeDuration, got: %v", err)
	}
}

func TestValidateMetric(t *testing.T) {
	m := &Metric{
		Ts:      time.Now().UTC(),
		Service: "ussd-gateway",
		Name:    "ussd_session_count",
		Value:   42.0,
	}
	if err := ValidateMetric(m); err != nil {
		t.Fatalf("expected valid metric, got: %v", err)
	}

	invalid := *m
	invalid.Value = math.NaN()
	if err := ValidateMetric(&invalid); err != ErrInvalidValue {
		t.Fatalf("expected ErrInvalidValue for NaN, got: %v", err)
	}
}

func TestValidateLog(t *testing.T) {
	l := &LogRecord{
		Service: "daraja-b2c",
		Body:    "STK push callback processed successfully",
	}
	if err := ValidateLog(l); err != nil {
		t.Fatalf("expected valid log, got: %v", err)
	}
	if l.Severity != "INFO" {
		t.Fatalf("expected default severity INFO, got: %s", l.Severity)
	}

	invalid := *l
	invalid.Body = "   "
	if err := ValidateLog(&invalid); err != ErrEmptyLogBody {
		t.Fatalf("expected ErrEmptyLogBody, got: %v", err)
	}
}

func TestBatcherFlushOnSize(t *testing.T) {
	var flushedItems int64
	var mu sync.Mutex

	batcher := NewBatcher[Span](
		"trace",
		100, // capacity
		5,   // maxBatchSize
		10*time.Second,
		func(ctx context.Context, batch []Span) error {
			mu.Lock()
			atomic.AddInt64(&flushedItems, int64(len(batch)))
			mu.Unlock()
			return nil
		},
	)
	defer batcher.Stop()

	for i := 0; i < 5; i++ {
		ok := batcher.Enqueue(Span{
			TraceID: "t1",
			SpanID:  "s1",
			Service: "test",
			Name:    "op",
		})
		if !ok {
			t.Fatalf("failed to enqueue item %d", i)
		}
	}

	// Wait up to 1s for the worker to process the size trigger
	time.Sleep(100 * time.Millisecond)

	count := atomic.LoadInt64(&flushedItems)
	if count != 5 {
		t.Fatalf("expected 5 flushed items, got %d", count)
	}
}

func TestBatcherFlushOnInterval(t *testing.T) {
	var flushedItems int64

	batcher := NewBatcher[Metric](
		"metric",
		100,
		1000,                  // high size threshold
		50*time.Millisecond,   // short ticker
		func(ctx context.Context, batch []Metric) error {
			atomic.AddInt64(&flushedItems, int64(len(batch)))
			return nil
		},
	)
	defer batcher.Stop()

	batcher.Enqueue(Metric{Service: "s1", Name: "m1", Value: 10})
	batcher.Enqueue(Metric{Service: "s1", Name: "m2", Value: 20})

	time.Sleep(150 * time.Millisecond)

	count := atomic.LoadInt64(&flushedItems)
	if count != 2 {
		t.Fatalf("expected 2 flushed items via ticker interval, got %d", count)
	}
}

func TestBatcherBackpressure(t *testing.T) {
	// Capacity 2, paused consumer (never flushes)
	batcher := NewBatcher[Span](
		"trace",
		2,
		100,
		1*time.Hour,
		func(ctx context.Context, batch []Span) error {
			return nil
		},
	)
	// Stop immediately to freeze queue consumption
	batcher.cancel()
	batcher.wg.Wait()

	// Fill the queue
	ok1 := batcher.Enqueue(Span{TraceID: "t1"})
	ok2 := batcher.Enqueue(Span{TraceID: "t2"})
	ok3 := batcher.Enqueue(Span{TraceID: "t3"}) // Must fail due to capacity 2

	if !ok1 || !ok2 {
		t.Fatal("expected first two enqueues to succeed")
	}
	if ok3 {
		t.Fatal("expected 3rd enqueue to fail with backpressure (queue full)")
	}
}

func TestHTTPReceiverEndpoints(t *testing.T) {
	traceBatcher := NewBatcher[Span]("trace", 100, 10, time.Second, func(ctx context.Context, batch []Span) error { return nil })
	metricBatcher := NewBatcher[Metric]("metric", 100, 10, time.Second, func(ctx context.Context, batch []Metric) error { return nil })
	logBatcher := NewBatcher[LogRecord]("log", 100, 10, time.Second, func(ctx context.Context, batch []LogRecord) error { return nil })
	defer traceBatcher.Stop()
	defer metricBatcher.Stop()
	defer logBatcher.Stop()

	receiver := NewHTTPReceiver(traceBatcher, metricBatcher, logBatcher)
	router := receiver.Routes()

	// 1. Send valid span array
	spans := []Span{
		{
			TraceID:    "54eba8423f6d4350ba3291c0bfd51e11",
			SpanID:     "91c0bfd51e110001",
			Service:    "daraja-b2c-gateway",
			Name:       "/v1/payments/b2c",
			StartTs:    time.Now().UTC(),
			DurationNs: 240000000,
		},
	}
	payload, _ := json.Marshal(spans)
	req := httptest.NewRequest("POST", "/v1/traces", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Send invalid span
	badSpans := []Span{{TraceID: ""}}
	badPayload, _ := json.Marshal(badSpans)
	reqBad := httptest.NewRequest("POST", "/v1/traces", bytes.NewReader(badPayload))
	wBad := httptest.NewRecorder()
	router.ServeHTTP(wBad, reqBad)

	if wBad.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for bad span, got %d", wBad.Code)
	}
}

