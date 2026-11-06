// path: api/ingest/validator.go
package ingest

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

var (
	ErrEmptyTraceID    = errors.New("trace_id must not be empty")
	ErrTraceIDTooLong  = errors.New("trace_id exceeds maximum length of 64 characters")
	ErrEmptySpanID     = errors.New("span_id must not be empty")
	ErrSpanIDTooLong   = errors.New("span_id exceeds maximum length of 32 characters")
	ErrEmptyService    = errors.New("service name must not be empty")
	ErrEmptySpanName   = errors.New("span name must not be empty")
	ErrZeroTimestamp   = errors.New("timestamp must not be zero")
	ErrNegativeDuration = errors.New("duration_ns must be greater than or equal to zero")
	ErrEmptyMetricName = errors.New("metric name must not be empty")
	ErrInvalidValue    = errors.New("metric value must be a finite number (not NaN or Inf)")
	ErrEmptyLogBody    = errors.New("log body must not be empty")
)

// ValidateSpan ensures all mandatory fields of a Span conform to storage schema requirements.
func ValidateSpan(s *Span) error {
	if s == nil {
		return errors.New("span pointer is nil")
	}
	s.TraceID = strings.TrimSpace(s.TraceID)
	if s.TraceID == "" {
		return ErrEmptyTraceID
	}
	if len(s.TraceID) > 64 {
		return ErrTraceIDTooLong
	}

	s.SpanID = strings.TrimSpace(s.SpanID)
	if s.SpanID == "" {
		return ErrEmptySpanID
	}
	if len(s.SpanID) > 32 {
		return ErrSpanIDTooLong
	}

	s.Service = strings.TrimSpace(s.Service)
	if s.Service == "" {
		return ErrEmptyService
	}

	s.Name = strings.TrimSpace(s.Name)
	if s.Name == "" {
		return ErrEmptySpanName
	}

	if s.StartTs.IsZero() {
		return ErrZeroTimestamp
	}

	if s.DurationNs < 0 {
		return ErrNegativeDuration
	}

	if s.Kind == "" {
		s.Kind = "SPAN_KIND_INTERNAL"
	}

	if s.Status == "" {
		s.Status = "STATUS_CODE_UNSET"
	}

	if s.Attrs == nil {
		s.Attrs = make(map[string]interface{})
	}

	return nil
}

// ValidateMetric validates a single time-series metric data point.
func ValidateMetric(m *Metric) error {
	if m == nil {
		return errors.New("metric pointer is nil")
	}
	m.Service = strings.TrimSpace(m.Service)
	if m.Service == "" {
		return ErrEmptyService
	}

	m.Name = strings.TrimSpace(m.Name)
	if m.Name == "" {
		return ErrEmptyMetricName
	}

	if m.Ts.IsZero() {
		return ErrZeroTimestamp
	}

	if math.IsNaN(m.Value) || math.IsInf(m.Value, 0) {
		return ErrInvalidValue
	}

	if m.Labels == nil {
		m.Labels = make(map[string]string)
	}

	return nil
}

// ValidateLog ensures a log record has valid timestamp, service, and body content.
func ValidateLog(l *LogRecord) error {
	if l == nil {
		return errors.New("log record pointer is nil")
	}
	l.Service = strings.TrimSpace(l.Service)
	if l.Service == "" {
		return ErrEmptyService
	}

	l.Body = strings.TrimSpace(l.Body)
	if l.Body == "" {
		return ErrEmptyLogBody
	}

	if l.Ts.IsZero() {
		l.Ts = time.Now().UTC()
	}

	if l.Severity == "" {
		l.Severity = "INFO"
	} else {
		l.Severity = strings.ToUpper(strings.TrimSpace(l.Severity))
	}

	if l.Attrs == nil {
		l.Attrs = make(map[string]interface{})
	}

	return nil
}

// ParseUnixNanoString parses an OTLP nanosecond string into time.Time.
func ParseUnixNanoString(nanoStr string) (time.Time, error) {
	if nanoStr == "" || nanoStr == "0" {
		return time.Now().UTC(), nil
	}
	var nanos int64
	_, err := fmt.Sscanf(nanoStr, "%d", &nanos)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid unix nano string %q: %w", nanoStr, err)
	}
	return time.Unix(0, nanos).UTC(), nil
}

