// path: api/db/writer.go
package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"observatory-apm/api/ingest"
)

// CopySpans bulk-inserts spans into the TimescaleDB hypertable using pgx.CopyFrom.
func CopySpans(ctx context.Context, pool *pgxpool.Pool, spans []ingest.Span) (int64, error) {
	if len(spans) == 0 {
		return 0, nil
	}
	if pool == nil {
		return 0, fmt.Errorf("database pool is nil")
	}

	rows := make([][]interface{}, len(spans))
	for i, s := range spans {
		attrsJSON, err := json.Marshal(s.Attrs)
		if err != nil {
			attrsJSON = []byte("{}")
		}

		rows[i] = []interface{}{
			s.StartTs,
			s.TraceID,
			s.SpanID,
			nullableString(s.ParentID),
			s.Service,
			s.Name,
			s.Kind,
			s.DurationNs,
			s.Status,
			attrsJSON,
		}
	}

	return pool.CopyFrom(
		ctx,
		pgx.Identifier{"spans"},
		[]string{"start_ts", "trace_id", "span_id", "parent_id", "service", "name", "kind", "duration_ns", "status", "attrs"},
		pgx.CopyFromRows(rows),
	)
}

// CopyMetrics bulk-inserts metrics into the TimescaleDB hypertable using pgx.CopyFrom.
func CopyMetrics(ctx context.Context, pool *pgxpool.Pool, metrics []ingest.Metric) (int64, error) {
	if len(metrics) == 0 {
		return 0, nil
	}
	if pool == nil {
		return 0, fmt.Errorf("database pool is nil")
	}

	rows := make([][]interface{}, len(metrics))
	for i, m := range metrics {
		labelsJSON, err := json.Marshal(m.Labels)
		if err != nil {
			labelsJSON = []byte("{}")
		}

		rows[i] = []interface{}{
			m.Ts,
			m.Service,
			m.Name,
			labelsJSON,
			m.Value,
		}
	}

	return pool.CopyFrom(
		ctx,
		pgx.Identifier{"metrics"},
		[]string{"ts", "service", "name", "labels", "value"},
		pgx.CopyFromRows(rows),
	)
}

// CopyLogs bulk-inserts log entries into the TimescaleDB hypertable using pgx.CopyFrom.
func CopyLogs(ctx context.Context, pool *pgxpool.Pool, logs []ingest.LogRecord) (int64, error) {
	if len(logs) == 0 {
		return 0, nil
	}
	if pool == nil {
		return 0, fmt.Errorf("database pool is nil")
	}

	rows := make([][]interface{}, len(logs))
	for i, l := range logs {
		attrsJSON, err := json.Marshal(l.Attrs)
		if err != nil {
			attrsJSON = []byte("{}")
		}

		rows[i] = []interface{}{
			l.Ts,
			l.Service,
			l.Severity,
			l.Body,
			nullableString(l.TraceID),
			nullableString(l.SpanID),
			attrsJSON,
		}
	}

	return pool.CopyFrom(
		ctx,
		pgx.Identifier{"logs"},
		[]string{"ts", "service", "severity", "body", "trace_id", "span_id", "attrs"},
		pgx.CopyFromRows(rows),
	)
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

