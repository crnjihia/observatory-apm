// path: api/ingest/receiver_http.go
package ingest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// SpanIngester defines an interface for sampling or staging spans before persistence.
type SpanIngester interface {
	Ingest(ctx context.Context, span Span) bool
}

// HTTPReceiver coordinates HTTP endpoints for OpenTelemetry ingestion.
type HTTPReceiver struct {
	traceBatcher  *Batcher[Span]
	metricBatcher *Batcher[Metric]
	logBatcher    *Batcher[LogRecord]
	tailSampler   SpanIngester
}

// NewHTTPReceiver creates an HTTP receiver backed by the respective batchers and optional tail sampler.
func NewHTTPReceiver(
	traceBatcher *Batcher[Span],
	metricBatcher *Batcher[Metric],
	logBatcher *Batcher[LogRecord],
	tailSampler ...SpanIngester,
) *HTTPReceiver {
	var sampler SpanIngester
	if len(tailSampler) > 0 {
		sampler = tailSampler[0]
	}
	return &HTTPReceiver{
		traceBatcher:  traceBatcher,
		metricBatcher: metricBatcher,
		logBatcher:    logBatcher,
		tailSampler:   sampler,
	}
}

// SetTailSampler dynamically attaches a tail-sampling engine.
func (h *HTTPReceiver) SetTailSampler(sampler SpanIngester) {
	h.tailSampler = sampler
}

// Routes mounts the OTLP/HTTP compliant ingestion endpoints onto a Chi subrouter.
func (h *HTTPReceiver) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/v1/traces", h.handleTraces)
	r.Post("/v1/metrics", h.handleMetrics)
	r.Post("/v1/logs", h.handleLogs)
	return r
}

func (h *HTTPReceiver) handleTraces(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		http.Error(w, `{"error":"failed to read request body"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var spans []Span

	// Try decoding as standard OTLP JSON export
	var otlp OTLPTracePayload
	if err := json.Unmarshal(body, &otlp); err == nil && len(otlp.ResourceSpans) > 0 {
		for _, rs := range otlp.ResourceSpans {
			service := extractResourceAttr(rs.Resource.Attributes, "service.name", "unknown-service")
			for _, ss := range rs.ScopeSpans {
				for _, s := range ss.Spans {
					startTs, _ := ParseUnixNanoString(s.StartTimeUnixNano)
					endTs, _ := ParseUnixNanoString(s.EndTimeUnixNano)
					duration := endTs.Sub(startTs).Nanoseconds()
					if duration < 0 {
						duration = 0
					}

					statusCode := "STATUS_CODE_UNSET"
					if s.Status.Code == 1 {
						statusCode = "STATUS_CODE_OK"
					} else if s.Status.Code == 2 {
						statusCode = "STATUS_CODE_ERROR"
					}

					attrs := make(map[string]interface{})
					for _, kv := range s.Attributes {
						attrs[kv.Key] = extractAnyValue(kv.Value)
					}

					spans = append(spans, Span{
						StartTs:    startTs,
						TraceID:    s.TraceID,
						SpanID:     s.SpanID,
						ParentID:   s.ParentSpanID,
						Service:    service,
						Name:       s.Name,
						Kind:       convertSpanKind(s.Kind),
						DurationNs: duration,
						Status:     statusCode,
						Attrs:      attrs,
					})
				}
			}
		}
	} else {
		// Fallback to direct array of Span
		var direct []Span
		if errDirect := json.Unmarshal(body, &direct); errDirect == nil && len(direct) > 0 {
			spans = direct
		} else {
			// Single object fallback
			var single Span
			if errSingle := json.Unmarshal(body, &single); errSingle == nil && single.TraceID != "" {
				spans = append(spans, single)
			}
		}
	}

	if len(spans) == 0 {
		http.Error(w, `{"error":"no valid spans found in request"}`, http.StatusBadRequest)
		return
	}

	// Validate & enqueue
	for i := range spans {
		if err := ValidateSpan(&spans[i]); err != nil {
			http.Error(w, `{"error":"validation failed: `+err.Error()+`"}`, http.StatusBadRequest)
			return
		}
	}

	for _, s := range spans {
		if h.tailSampler != nil {
			if ok := h.tailSampler.Ingest(r.Context(), s); !ok {
				w.Header().Set("Retry-After", "1")
				http.Error(w, `{"error":"ingest queue full, backpressure applied"}`, http.StatusTooManyRequests)
				return
			}
		} else {
			if ok := h.traceBatcher.Enqueue(s); !ok {
				w.Header().Set("Retry-After", "1")
				http.Error(w, `{"error":"ingest queue full, backpressure applied"}`, http.StatusTooManyRequests)
				return
			}
		}
	}

	IngestReceivedTotal.WithLabelValues("trace", "accepted").Inc()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"partialSuccess":{}}`))
}

func (h *HTTPReceiver) handleMetrics(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024))
	if err != nil {
		http.Error(w, `{"error":"failed to read request body"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var metrics []Metric

	var otlp OTLPMetricPayload
	if err := json.Unmarshal(body, &otlp); err == nil && len(otlp.ResourceMetrics) > 0 {
		for _, rm := range otlp.ResourceMetrics {
			service := extractResourceAttr(rm.Resource.Attributes, "service.name", "unknown-service")
			for _, sm := range rm.ScopeMetrics {
				for _, m := range sm.Metrics {
					if m.Gauge != nil {
						for _, dp := range m.Gauge.DataPoints {
							ts, _ := ParseUnixNanoString(dp.TimeUnixNano)
							val := extractDataPointValue(dp)
							labels := make(map[string]string)
							for _, kv := range dp.Attributes {
								labels[kv.Key] = stringifyAnyValue(kv.Value)
							}
							metrics = append(metrics, Metric{
								Ts:      ts,
								Service: service,
								Name:    m.Name,
								Labels:  labels,
								Value:   val,
							})
						}
					}
					if m.Sum != nil {
						for _, dp := range m.Sum.DataPoints {
							ts, _ := ParseUnixNanoString(dp.TimeUnixNano)
							val := extractDataPointValue(dp)
							labels := make(map[string]string)
							for _, kv := range dp.Attributes {
								labels[kv.Key] = stringifyAnyValue(kv.Value)
							}
							metrics = append(metrics, Metric{
								Ts:      ts,
								Service: service,
								Name:    m.Name,
								Labels:  labels,
								Value:   val,
							})
						}
					}
				}
			}
		}
	} else {
		var direct []Metric
		if errDirect := json.Unmarshal(body, &direct); errDirect == nil && len(direct) > 0 {
			metrics = direct
		} else {
			var single Metric
			if errSingle := json.Unmarshal(body, &single); errSingle == nil && single.Name != "" {
				metrics = append(metrics, single)
			}
		}
	}

	if len(metrics) == 0 {
		http.Error(w, `{"error":"no valid metrics found in request"}`, http.StatusBadRequest)
		return
	}

	for i := range metrics {
		if err := ValidateMetric(&metrics[i]); err != nil {
			http.Error(w, `{"error":"validation failed: `+err.Error()+`"}`, http.StatusBadRequest)
			return
		}
	}

	for _, m := range metrics {
		if ok := h.metricBatcher.Enqueue(m); !ok {
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":"ingest queue full, backpressure applied"}`, http.StatusTooManyRequests)
			return
		}
	}

	IngestReceivedTotal.WithLabelValues("metric", "accepted").Inc()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"partialSuccess":{}}`))
}

func (h *HTTPReceiver) handleLogs(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024))
	if err != nil {
		http.Error(w, `{"error":"failed to read request body"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var logs []LogRecord

	var otlp OTLPLogPayload
	if err := json.Unmarshal(body, &otlp); err == nil && len(otlp.ResourceLogs) > 0 {
		for _, rl := range otlp.ResourceLogs {
			service := extractResourceAttr(rl.Resource.Attributes, "service.name", "unknown-service")
			for _, sl := range rl.ScopeLogs {
				for _, lr := range sl.LogRecords {
					ts, _ := ParseUnixNanoString(lr.TimeUnixNano)
					attrs := make(map[string]interface{})
					for _, kv := range lr.Attributes {
						attrs[kv.Key] = extractAnyValue(kv.Value)
					}
					severity := lr.SeverityText
					if severity == "" {
						severity = severityNumberToText(lr.SeverityNumber)
					}

					logs = append(logs, LogRecord{
						Ts:       ts,
						Service:  service,
						Severity: severity,
						Body:     lr.Body.StringValue,
						TraceID:  lr.TraceID,
						SpanID:   lr.SpanID,
						Attrs:    attrs,
					})
				}
			}
		}
	} else {
		var direct []LogRecord
		if errDirect := json.Unmarshal(body, &direct); errDirect == nil && len(direct) > 0 {
			logs = direct
		} else {
			var single LogRecord
			if errSingle := json.Unmarshal(body, &single); errSingle == nil && single.Body != "" {
				logs = append(logs, single)
			}
		}
	}

	if len(logs) == 0 {
		http.Error(w, `{"error":"no valid logs found in request"}`, http.StatusBadRequest)
		return
	}

	for i := range logs {
		if err := ValidateLog(&logs[i]); err != nil {
			http.Error(w, `{"error":"validation failed: `+err.Error()+`"}`, http.StatusBadRequest)
			return
		}
	}

	for _, l := range logs {
		if ok := h.logBatcher.Enqueue(l); !ok {
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":"ingest queue full, backpressure applied"}`, http.StatusTooManyRequests)
			return
		}
	}

	IngestReceivedTotal.WithLabelValues("log", "accepted").Inc()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"partialSuccess":{}}`))
}

// Helpers for extracting attributes
func extractResourceAttr(attrs []KeyValue, key, fallback string) string {
	for _, kv := range attrs {
		if kv.Key == key && kv.Value.StringValue != nil {
			return *kv.Value.StringValue
		}
	}
	return fallback
}

func extractAnyValue(v AnyValue) interface{} {
	if v.StringValue != nil {
		return *v.StringValue
	}
	if v.DoubleValue != nil {
		return *v.DoubleValue
	}
	if v.BoolValue != nil {
		return *v.BoolValue
	}
	if v.IntValue != nil {
		if i, err := strconv.ParseInt(*v.IntValue, 10, 64); err == nil {
			return i
		}
		return *v.IntValue
	}
	return nil
}

func stringifyAnyValue(v AnyValue) string {
	if v.StringValue != nil {
		return *v.StringValue
	}
	if v.IntValue != nil {
		return *v.IntValue
	}
	if v.DoubleValue != nil {
		return strconv.FormatFloat(*v.DoubleValue, 'f', -1, 64)
	}
	if v.BoolValue != nil {
		return strconv.FormatBool(*v.BoolValue)
	}
	return ""
}

func extractDataPointValue(dp DataPoint) float64 {
	if dp.AsDouble != nil {
		return *dp.AsDouble
	}
	if dp.AsInt != nil {
		if f, err := strconv.ParseFloat(*dp.AsInt, 64); err == nil {
			return f
		}
	}
	return 0.0
}

func convertSpanKind(k int) string {
	switch k {
	case 1:
		return "SPAN_KIND_INTERNAL"
	case 2:
		return "SPAN_KIND_SERVER"
	case 3:
		return "SPAN_KIND_CLIENT"
	case 4:
		return "SPAN_KIND_PRODUCER"
	case 5:
		return "SPAN_KIND_CONSUMER"
	default:
		return "SPAN_KIND_INTERNAL"
	}
}

func severityNumberToText(num int) string {
	switch {
	case num >= 1 && num <= 4:
		return "TRACE"
	case num >= 5 && num <= 8:
		return "DEBUG"
	case num >= 9 && num <= 12:
		return "INFO"
	case num >= 13 && num <= 16:
		return "WARN"
	case num >= 17 && num <= 20:
		return "ERROR"
	case num >= 21 && num <= 24:
		return "FATAL"
	default:
		return "INFO"
	}
}
