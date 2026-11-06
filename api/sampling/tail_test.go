// path: api/sampling/tail_test.go
package sampling

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"observatory-apm/api/ingest"
)

// mockSink records spans enqueued by the tail sampler.
type mockSink struct {
	mu    sync.Mutex
	spans []ingest.Span
}

func newMockSink() *mockSink {
	return &mockSink{spans: make([]ingest.Span, 0)}
}

func (m *mockSink) Enqueue(span ingest.Span) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spans = append(m.spans, span)
	return true
}

func (m *mockSink) Spans() []ingest.Span {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]ingest.Span, len(m.spans))
	copy(copied, m.spans)
	return copied
}

func (m *mockSink) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.spans)
}

func TestHeadSampler(t *testing.T) {
	// Boundary ratio 0.0: all dropped
	s0 := NewHeadSampler(0.0)
	if dec := s0.TraceIdRatioDecision("4bf92f3577b34da6a3ce929d0e0e4736"); dec != DecisionDrop {
		t.Fatalf("expected DecisionDrop for ratio 0.0, got %v", dec)
	}

	// Boundary ratio 1.0: all sampled
	s1 := NewHeadSampler(1.0)
	if dec := s1.TraceIdRatioDecision("4bf92f3577b34da6a3ce929d0e0e4736"); dec != DecisionRecordAndSample {
		t.Fatalf("expected DecisionRecordAndSample for ratio 1.0, got %v", dec)
	}

	// 10% ratio test across sample pool
	s10 := NewHeadSampler(0.10)
	sampledCount := 0
	totalTraces := 2000
	for i := 0; i < totalTraces; i++ {
		traceID := fmt.Sprintf("0000000000000000%016x", i)
		if s10.TraceIdRatioDecision(traceID) == DecisionRecordAndSample {
			sampledCount++
		}
	}
	samplePercentage := float64(sampledCount) / float64(totalTraces) * 100
	if samplePercentage < 5.0 || samplePercentage > 15.0 {
		t.Errorf("expected 10%% ratio (+/- 5%%), got %.2f%% (%d/%d)", samplePercentage, sampledCount, totalTraces)
	}

	// ParentBased rules
	// 1. Root span
	if s10.ShouldSample("ffffffffffffffffffffffffffffffff", false, false) != DecisionDrop {
		t.Errorf("expected drop for high traceId root span")
	}
	// 2. Sampled parent
	if dec := s10.ShouldSample("ffffffffffffffffffffffffffffffff", true, true); dec != DecisionRecordAndSample {
		t.Errorf("expected DecisionRecordAndSample when parent sampled, got %v", dec)
	}
	// 3. Unsampled parent
	if dec := s10.ShouldSample("00000000000000000000000000000001", true, false); dec != DecisionDrop {
		t.Errorf("expected DecisionDrop when parent not sampled, got %v", dec)
	}
}

func TestParseTraceparent(t *testing.T) {
	// Valid sampled header
	h1 := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	tID, sID, sampled, ok := ParseTraceparent(h1)
	if !ok || !sampled || tID != "4bf92f3577b34da6a3ce929d0e0e4736" || sID != "00f067aa0ba902b7" {
		t.Fatalf("failed to parse valid sampled traceparent: %s, ok=%v, sampled=%v", h1, ok, sampled)
	}

	// Valid unsampled header
	h2 := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00"
	_, _, sampled, ok = ParseTraceparent(h2)
	if !ok || sampled {
		t.Fatalf("expected unsampled traceparent: %s", h2)
	}

	// Malformed headers
	malformed := []string{
		"",
		"invalid",
		"01-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		"00-short-00f067aa0ba902b7-01",
	}
	for _, m := range malformed {
		if _, _, _, ok := ParseTraceparent(m); ok {
			t.Errorf("expected parse failure for %s", m)
		}
	}
}

func TestMemoryTraceBuffer(t *testing.T) {
	ctx := context.Background()
	buf := NewMemoryTraceBuffer()
	defer buf.Close()

	traceID := "test-trace-12345"
	s1 := ingest.Span{
		TraceID:    traceID,
		SpanID:     "span-1",
		Service:    "payment-service",
		Name:       "ProcessMpesaCallback",
		DurationNs: 45_000_000,
		Status:     "STATUS_CODE_OK",
	}
	s2 := ingest.Span{
		TraceID:    traceID,
		SpanID:     "span-2",
		ParentID:   "span-1",
		Service:    "notification-service",
		Name:       "SendSMSConfirmation",
		DurationNs: 20_000_000,
		Status:     "STATUS_CODE_OK",
	}

	if err := buf.AddSpan(ctx, s1); err != nil {
		t.Fatalf("AddSpan s1 failed: %v", err)
	}
	if err := buf.AddSpan(ctx, s2); err != nil {
		t.Fatalf("AddSpan s2 failed: %v", err)
	}

	count, _ := buf.ActiveTraceCount(ctx)
	if count != 1 {
		t.Fatalf("expected 1 active trace, got %d", count)
	}

	spans, err := buf.GetSpans(ctx, traceID)
	if err != nil || len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d (err: %v)", len(spans), err)
	}

	// Cutoff check
	futureCutoff := time.Now().Add(1 * time.Hour)
	expired, err := buf.GetExpiredTraces(ctx, futureCutoff)
	if err != nil || len(expired) != 1 || expired[0] != traceID {
		t.Fatalf("expected trace %s to be expired, got %v", traceID, expired)
	}

	pastCutoff := time.Now().Add(-1 * time.Hour)
	expiredPast, _ := buf.GetExpiredTraces(ctx, pastCutoff)
	if len(expiredPast) != 0 {
		t.Fatalf("expected 0 expired traces for past cutoff, got %d", len(expiredPast))
	}

	// Delete
	if err := buf.Delete(ctx, traceID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	countAfter, _ := buf.ActiveTraceCount(ctx)
	if countAfter != 0 {
		t.Fatalf("expected 0 active traces after delete, got %d", countAfter)
	}
}

func TestTailEvaluator_ErrorTrace(t *testing.T) {
	ctx := context.Background()
	buf := NewMemoryTraceBuffer()
	defer buf.Close()

	sink := newMockSink()
	cfg := TailSamplerConfig{
		DecisionWindow:  50 * time.Millisecond,
		SlowThresholdMs: 1500,
		HeadSampleRate:  0.0, // Force drop of normal traces
		CacheTTL:        10 * time.Second,
	}
	sampler := NewTailSampler(cfg, buf, sink)

	traceID := "error-trace-001"
	span1 := ingest.Span{
		TraceID:    traceID,
		SpanID:     "s1",
		Service:    "daraja-gateway",
		Name:       "STKPushCallback",
		DurationNs: 80_000_000,
		Status:     "STATUS_CODE_OK",
	}
	span2 := ingest.Span{
		TraceID:    traceID,
		SpanID:     "s2",
		ParentID:   "s1",
		Service:    "ledger-service",
		Name:       "DebitAccount",
		DurationNs: 35_000_000,
		Status:     "STATUS_CODE_ERROR", // Error span!
	}

	sampler.Ingest(ctx, span1)
	sampler.Ingest(ctx, span2)

	// Wait past decision window
	time.Sleep(100 * time.Millisecond)
	processed, err := sampler.ProcessExpiredTraces(ctx)
	if err != nil {
		t.Fatalf("ProcessExpiredTraces failed: %v", err)
	}
	if processed != 1 {
		t.Fatalf("expected 1 trace processed, got %d", processed)
	}

	if sink.Count() != 2 {
		t.Fatalf("expected 2 error trace spans kept in sink, got %d", sink.Count())
	}
}

func TestTailEvaluator_SlowRootSpan(t *testing.T) {
	ctx := context.Background()
	buf := NewMemoryTraceBuffer()
	defer buf.Close()

	sink := newMockSink()
	cfg := TailSamplerConfig{
		DecisionWindow:  50 * time.Millisecond,
		SlowThresholdMs: 1500, // 1500 ms threshold
		HeadSampleRate:  0.0,  // Drop normal traces
		CacheTTL:        10 * time.Second,
	}
	sampler := NewTailSampler(cfg, buf, sink)

	traceID := "slow-root-trace-002"
	span1 := ingest.Span{
		TraceID:    traceID,
		SpanID:     "root-1",
		Service:    "ussd-gateway",
		Name:       "SessionMenuRender",
		DurationNs: 1800 * 1_000_000, // 1800 ms (slow root span)
		Status:     "STATUS_CODE_OK",
	}

	sampler.Ingest(ctx, span1)

	time.Sleep(100 * time.Millisecond)
	processed, err := sampler.ProcessExpiredTraces(ctx)
	if err != nil {
		t.Fatalf("ProcessExpiredTraces error: %v", err)
	}
	if processed != 1 {
		t.Fatalf("expected 1 trace processed, got %d", processed)
	}

	if sink.Count() != 1 {
		t.Fatalf("expected slow root span to be 100%% kept, got %d", sink.Count())
	}
}

func TestTailEvaluator_FastNormalSpanDropped(t *testing.T) {
	ctx := context.Background()
	buf := NewMemoryTraceBuffer()
	defer buf.Close()

	sink := newMockSink()
	cfg := TailSamplerConfig{
		DecisionWindow:  50 * time.Millisecond,
		SlowThresholdMs: 1500,
		HeadSampleRate:  0.0, // Force drop of normal fast traces
		CacheTTL:        10 * time.Second,
	}
	sampler := NewTailSampler(cfg, buf, sink)

	traceID := "fast-normal-trace-003"
	span1 := ingest.Span{
		TraceID:    traceID,
		SpanID:     "s1",
		Service:    "auth-service",
		Name:       "VerifyToken",
		DurationNs: 15 * 1_000_000, // 15ms fast span
		Status:     "STATUS_CODE_OK",
	}

	sampler.Ingest(ctx, span1)

	time.Sleep(100 * time.Millisecond)
	processed, _ := sampler.ProcessExpiredTraces(ctx)
	if processed != 1 {
		t.Fatalf("expected 1 trace processed, got %d", processed)
	}

	if sink.Count() != 0 {
		t.Fatalf("expected fast normal trace to be dropped, got %d in sink", sink.Count())
	}
}

func TestTailEvaluator_LateArrivingSpans(t *testing.T) {
	ctx := context.Background()
	buf := NewMemoryTraceBuffer()
	defer buf.Close()

	sink := newMockSink()
	cfg := TailSamplerConfig{
		DecisionWindow:  50 * time.Millisecond,
		SlowThresholdMs: 1500,
		HeadSampleRate:  0.0,
		CacheTTL:        5 * time.Second,
	}
	sampler := NewTailSampler(cfg, buf, sink)

	traceID := "late-arriving-kept-004"
	rootSpan := ingest.Span{
		TraceID:    traceID,
		SpanID:     "root",
		Service:    "daraja-api",
		Name:       "B2CPayment",
		DurationNs: 2200 * 1_000_000, // Slow root -> Keep decision
		Status:     "STATUS_CODE_OK",
	}

	// 1. Ingest root span and let decision expire
	sampler.Ingest(ctx, rootSpan)
	time.Sleep(100 * time.Millisecond)
	_, _ = sampler.ProcessExpiredTraces(ctx)

	if sink.Count() != 1 {
		t.Fatalf("expected root span in sink, got %d", sink.Count())
	}

	// 2. Late arriving child span arrives after decision was made
	lateChild := ingest.Span{
		TraceID:    traceID,
		SpanID:     "child-late",
		ParentID:   "root",
		Service:    "sms-provider",
		Name:       "SendAfricasTalkingSMS",
		DurationNs: 50 * 1_000_000,
		Status:     "STATUS_CODE_OK",
	}

	sampler.Ingest(ctx, lateChild)

	// Since trace was decided "Keep", late child should immediately be passed to sink
	if sink.Count() != 2 {
		t.Fatalf("expected late arriving span to be accepted into sink immediately, got %d", sink.Count())
	}
	if sink.Spans()[1].SpanID != "child-late" {
		t.Fatalf("expected late span ID child-late, got %s", sink.Spans()[1].SpanID)
	}

	// 3. Test late arriving span for dropped trace
	droppedTraceID := "late-arriving-drop-005"
	fastSpan := ingest.Span{
		TraceID:    droppedTraceID,
		SpanID:     "fast",
		Service:    "health",
		Name:       "Ping",
		DurationNs: 2 * 1_000_000,
		Status:     "STATUS_CODE_OK",
	}
	sampler.Ingest(ctx, fastSpan)
	time.Sleep(100 * time.Millisecond)
	_, _ = sampler.ProcessExpiredTraces(ctx)

	lateFastChild := ingest.Span{
		TraceID:    droppedTraceID,
		SpanID:     "fast-child",
		ParentID:   "fast",
		Service:    "health",
		Name:       "PingSub",
		DurationNs: 1 * 1_000_000,
		Status:     "STATUS_CODE_OK",
	}
	sampler.Ingest(ctx, lateFastChild)

	// Total count in sink should still remain 2 (the late child of dropped trace was discarded)
	if sink.Count() != 2 {
		t.Fatalf("expected late span of dropped trace to be discarded, got count %d", sink.Count())
	}
}

