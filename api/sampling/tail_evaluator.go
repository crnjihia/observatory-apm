// path: api/sampling/tail_evaluator.go
package sampling

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"observatory-apm/api/ingest"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Prometheus metrics for tail sampling observability.
	samplingEvaluatedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "observatory_sampling_evaluated_total",
			Help: "Count of trace tail-sampling decisions partitioned by decision and reason.",
		},
		[]string{"decision", "reason"},
	)

	samplingLateSpansTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "observatory_sampling_late_spans_total",
			Help: "Count of late-arriving spans arriving after initial trace decision.",
		},
		[]string{"decision"},
	)

	samplingDecisionDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "observatory_sampling_decision_duration_seconds",
			Help:    "Execution duration of tail sampling evaluation sweeps in seconds.",
			Buckets: prometheus.DefBuckets,
		},
	)
)

var metricsOnce sync.Once

func registerSamplingMetrics() {
	metricsOnce.Do(func() {
		_ = prometheus.Register(samplingEvaluatedTotal)
		_ = prometheus.Register(samplingLateSpansTotal)
		_ = prometheus.Register(samplingDecisionDuration)
	})
}

// SpanSink defines the consumer destination for spans that pass tail sampling.
type SpanSink interface {
	Enqueue(span ingest.Span) bool
}

// TailSamplerConfig defines timing and decision thresholds for tail sampling.
type TailSamplerConfig struct {
	DecisionWindow  time.Duration // Default: 3 seconds
	SlowThresholdMs int64         // Default: 1500 ms (Kenyan Daraja/USSD SLA threshold)
	HeadSampleRate  float64       // Default: 0.1 (10% ratio for normal traces)
	CacheTTL        time.Duration // Default: 60 seconds
}

// DefaultTailSamplerConfig returns production default configurations.
func DefaultTailSamplerConfig() TailSamplerConfig {
	return TailSamplerConfig{
		DecisionWindow:  3 * time.Second,
		SlowThresholdMs: 1500,
		HeadSampleRate:  0.10,
		CacheTTL:        60 * time.Second,
	}
}

type cachedDecision struct {
	decision Decision
	expires  time.Time
}

// TailSampler coordinates sliding-window buffering, late-span routing,
// and tail-sampling heuristics (100% errors, 100% slow root spans, 10% normal).
type TailSampler struct {
	cfg         TailSamplerConfig
	buffer      TraceBuffer
	headSampler *HeadSampler
	sink        SpanSink

	mu            sync.RWMutex
	decisionCache map[string]cachedDecision

	stopCh chan struct{}
	doneCh chan struct{}
}

// NewTailSampler constructs and initializes a TailSampler.
func NewTailSampler(cfg TailSamplerConfig, buffer TraceBuffer, sink SpanSink) *TailSampler {
	registerSamplingMetrics()

	if cfg.DecisionWindow <= 0 {
		cfg.DecisionWindow = 3 * time.Second
	}
	if cfg.SlowThresholdMs <= 0 {
		cfg.SlowThresholdMs = 1500
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = 60 * time.Second
	}

	ts := &TailSampler{
		cfg:           cfg,
		buffer:        buffer,
		headSampler:   NewHeadSampler(cfg.HeadSampleRate),
		sink:          sink,
		decisionCache: make(map[string]cachedDecision),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}

	return ts
}

// Start spawns the background decision evaluator loop.
func (ts *TailSampler) Start() {
	go ts.evaluatorLoop()
}

// Ingest handles incoming spans:
// 1. If trace was previously decided:
//    - If Keep: immediately forges span to downstream sink.
//    - If Drop: immediately discards span.
// 2. If trace is undecided: appends span to active window buffer.
func (ts *TailSampler) Ingest(ctx context.Context, span ingest.Span) bool {
	ts.mu.RLock()
	cached, found := ts.decisionCache[span.TraceID]
	ts.mu.RUnlock()

	if found && time.Now().Before(cached.expires) {
		if cached.decision == DecisionRecordAndSample {
			samplingLateSpansTotal.WithLabelValues("keep").Inc()
			if ts.sink != nil {
				return ts.sink.Enqueue(span)
			}
			return true
		}
		samplingLateSpansTotal.WithLabelValues("drop").Inc()
		return true // Accepted and discarded as per decision
	}

	// Buffer span in active decision window
	if err := ts.buffer.AddSpan(ctx, span); err != nil {
		log.Printf("[TailSampler] Failed to buffer span for trace %s: %v", span.TraceID, err)
		// Fallback: enqueue immediately if buffering fails so we don't lose data
		if ts.sink != nil {
			return ts.sink.Enqueue(span)
		}
		return false
	}
	return true
}

// EvaluateTrace applies the tail sampling heuristics to a set of spans belonging to one trace.
// Returns the sampling decision and human-readable reason.
func (ts *TailSampler) EvaluateTrace(spans []ingest.Span) (Decision, string) {
	if len(spans) == 0 {
		return DecisionDrop, "empty_trace"
	}

	slowThresholdNs := ts.cfg.SlowThresholdMs * 1_000_000

	// Heuristic 1: Any span with ERROR status -> Keep 100%
	for _, s := range spans {
		if s.Status == "STATUS_CODE_ERROR" {
			return DecisionRecordAndSample, "error_status"
		}
		// Inspect HTTP or error attributes
		if code, ok := s.Attrs["http.status_code"]; ok {
			switch c := code.(type) {
			case float64:
				if c >= 500 {
					return DecisionRecordAndSample, "http_5xx"
				}
			case int:
				if c >= 500 {
					return DecisionRecordAndSample, "http_5xx"
				}
			}
		}
		if isErr, ok := s.Attrs["error"].(bool); ok && isErr {
			return DecisionRecordAndSample, "error_attr"
		}
	}

	// Heuristic 2: Slow root span (> threshold_ms) -> Keep 100%
	for _, s := range spans {
		if s.ParentID == "" {
			if s.DurationNs >= slowThresholdNs {
				return DecisionRecordAndSample, "slow_root_span"
			}
		}
	}

	// Heuristic 3: Any slow child span exceeding threshold -> Keep 100%
	for _, s := range spans {
		if s.DurationNs >= slowThresholdNs {
			return DecisionRecordAndSample, "slow_child_span"
		}
	}

	// Heuristic 4: Normal trace -> apply head ratio (e.g. 10%)
	traceID := spans[0].TraceID
	ratioDecision := ts.headSampler.TraceIdRatioDecision(traceID)
	if ratioDecision == DecisionRecordAndSample {
		return DecisionRecordAndSample, "head_ratio_sample"
	}

	return DecisionDrop, "normal_unsampled"
}

// ProcessExpiredTraces sweeps all traces whose decision window has elapsed.
func (ts *TailSampler) ProcessExpiredTraces(ctx context.Context) (int, error) {
	start := time.Now()
	cutoff := start.Add(-ts.cfg.DecisionWindow)

	expiredIDs, err := ts.buffer.GetExpiredTraces(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch expired traces: %w", err)
	}

	now := time.Now()
	cacheExpiry := now.Add(ts.cfg.CacheTTL)
	processed := 0

	for _, traceID := range expiredIDs {
		spans, err := ts.buffer.GetSpans(ctx, traceID)
		if err != nil || len(spans) == 0 {
			_ = ts.buffer.Delete(ctx, traceID)
			continue
		}

		decision, reason := ts.EvaluateTrace(spans)

		// Cache decision for late-arriving spans
		ts.mu.Lock()
		ts.decisionCache[traceID] = cachedDecision{
			decision: decision,
			expires:  cacheExpiry,
		}
		ts.mu.Unlock()

		if decision == DecisionRecordAndSample {
			samplingEvaluatedTotal.WithLabelValues("keep", reason).Inc()
			if ts.sink != nil {
				for _, span := range spans {
					_ = ts.sink.Enqueue(span)
				}
			}
		} else {
			samplingEvaluatedTotal.WithLabelValues("drop", reason).Inc()
		}

		_ = ts.buffer.Delete(ctx, traceID)
		processed++
	}

	// Prune expired entries from decision cache
	ts.pruneDecisionCache(now)

	samplingDecisionDuration.Observe(time.Since(start).Seconds())
	return processed, nil
}

func (ts *TailSampler) pruneDecisionCache(now time.Time) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	for k, v := range ts.decisionCache {
		if now.After(v.expires) {
			delete(ts.decisionCache, k)
		}
	}
}

func (ts *TailSampler) evaluatorLoop() {
	defer close(ts.doneCh)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ts.stopCh:
			// Final flush on shutdown
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, _ = ts.ProcessExpiredTraces(ctx)
			cancel()
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_, err := ts.ProcessExpiredTraces(ctx)
			if err != nil {
				log.Printf("[TailSampler] Sweep error: %v", err)
			}
			cancel()
		}
	}
}

// Stop terminates the background evaluator and flushes active buffers.
func (ts *TailSampler) Stop() {
	close(ts.stopCh)
	<-ts.doneCh
	_ = ts.buffer.Close()
}

