// path: packages/agent-go/logger.go
package observatory

import (
	"context"
	"log/slog"
)

// TraceHandler wraps an slog.Handler to inject active trace_id and span_id into log records.
type TraceHandler struct {
	slog.Handler
}

// NewTraceHandler wraps an existing slog handler with trace correlation capabilities.
func NewTraceHandler(h slog.Handler) *TraceHandler {
	return &TraceHandler{Handler: h}
}

// Handle extracts trace context and appends trace_id and span_id attributes.
func (h *TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	if tc, ok := FromContext(ctx); ok {
		if tc.TraceID != "" {
			r.AddAttrs(slog.String("trace_id", tc.TraceID))
		}
		if tc.SpanID != "" {
			r.AddAttrs(slog.String("span_id", tc.SpanID))
		}
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs returns a new handler whose attributes include the given attributes.
func (h *TraceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup returns a new handler with the given group name.
func (h *TraceHandler) WithGroup(name string) slog.Handler {
	return &TraceHandler{Handler: h.Handler.WithGroup(name)}
}

// NewCorrelatedLogger returns a configured *slog.Logger that automatically attaches trace IDs.
func NewCorrelatedLogger(h slog.Handler) *slog.Logger {
	return slog.New(NewTraceHandler(h))
}

