// path: api/ingest/types.go
package ingest

import (
	"time"
)

// Span represents a normalized distributed trace span stored in TimescaleDB.
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
}

// Metric represents a single time-series metric data point.
type Metric struct {
	Ts     time.Time         `json:"ts"`
	Service string            `json:"service"`
	Name    string            `json:"name"`
	Labels  map[string]string `json:"labels"`
	Value   float64           `json:"value"`
}

// LogRecord represents an application log correlated with a trace.
type LogRecord struct {
	Ts       time.Time              `json:"ts"`
	Service  string                 `json:"service"`
	Severity string                 `json:"severity"`
	Body     string                 `json:"body"`
	TraceID  string                 `json:"trace_id,omitempty"`
	SpanID   string                 `json:"span_id,omitempty"`
	Attrs    map[string]interface{} `json:"attrs"`
}

// ============================================================================
// OTLP JSON Ingest DTOs (compatible with OpenTelemetry HTTP/JSON protocol)
// ============================================================================

type OTLPTracePayload struct {
	ResourceSpans []ResourceSpan `json:"resourceSpans"`
}

type ResourceSpan struct {
	Resource   Resource     `json:"resource"`
	ScopeSpans []ScopeSpan  `json:"scopeSpans"`
}

type Resource struct {
	Attributes []KeyValue `json:"attributes"`
}

type ScopeSpan struct {
	Spans []OTLPSpan `json:"spans"`
}

type OTLPSpan struct {
	TraceID           string     `json:"traceId"`
	SpanID            string     `json:"spanId"`
	ParentSpanID      string     `json:"parentSpanId,omitempty"`
	Name              string     `json:"name"`
	Kind              int        `json:"kind"`
	StartTimeUnixNano string     `json:"startTimeUnixNano"`
	EndTimeUnixNano   string     `json:"endTimeUnixNano"`
	Attributes        []KeyValue `json:"attributes"`
	Status            OTLPStatus `json:"status"`
}

type OTLPStatus struct {
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}

type OTLPMetricPayload struct {
	ResourceMetrics []ResourceMetric `json:"resourceMetrics"`
}

type ResourceMetric struct {
	Resource     Resource      `json:"resource"`
	ScopeMetrics []ScopeMetric `json:"scopeMetrics"`
}

type ScopeMetric struct {
	Metrics []OTLPMetric `json:"metrics"`
}

type OTLPMetric struct {
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Unit        string     `json:"unit,omitempty"`
	Gauge       *GaugeData `json:"gauge,omitempty"`
	Sum         *SumData   `json:"sum,omitempty"`
}

type GaugeData struct {
	DataPoints []DataPoint `json:"dataPoints"`
}

type SumData struct {
	DataPoints []DataPoint `json:"dataPoints"`
}

type DataPoint struct {
	TimeUnixNano   string     `json:"timeUnixNano"`
	AsDouble       *float64   `json:"asDouble,omitempty"`
	AsInt          *string    `json:"asInt,omitempty"`
	Attributes     []KeyValue `json:"attributes"`
}

type OTLPLogPayload struct {
	ResourceLogs []ResourceLog `json:"resourceLogs"`
}

type ResourceLog struct {
	Resource  Resource   `json:"resource"`
	ScopeLogs []ScopeLog `json:"scopeLogs"`
}

type ScopeLog struct {
	LogRecords []OTLPLogRecord `json:"logRecords"`
}

type OTLPLogRecord struct {
	TimeUnixNano   string     `json:"timeUnixNano"`
	SeverityText   string     `json:"severityText"`
	SeverityNumber int        `json:"severityNumber"`
	Body           LogValue   `json:"body"`
	Attributes     []KeyValue `json:"attributes"`
	TraceID        string     `json:"traceId,omitempty"`
	SpanID         string     `json:"spanId,omitempty"`
}

type LogValue struct {
	StringValue string `json:"stringValue"`
}

type KeyValue struct {
	Key   string    `json:"key"`
	Value AnyValue  `json:"value"`
}

type AnyValue struct {
	StringValue *string  `json:"stringValue,omitempty"`
	IntValue    *string  `json:"intValue,omitempty"`
	DoubleValue *float64 `json:"doubleValue,omitempty"`
	BoolValue   *bool    `json:"boolValue,omitempty"`
}

