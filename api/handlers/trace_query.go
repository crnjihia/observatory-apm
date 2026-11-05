// path: C:/Users/crnji/Videos/Projects/Web Development/New/observatory-apm/api/handlers/trace_query.go
package handlers

import (
    "encoding/json"
    "net/http"
    "strconv"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/jackc/pgx/v5"
)

// TraceIDResponse is a slice of trace IDs.
type TraceIDResponse []string

// GetTracesByBucket returns distinct trace IDs for a service/endpoint within a minute bucket.
// Query parameters:
//   service (required)
//   endpoint (optional)
//   bucket (required) – RFC3339 minute timestamp (e.g., 2026-11-05T12:00:00Z)
//   limit (optional, default 20, max 100)
func GetTracesByBucket(pool *pgxpool.Pool) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        qs := r.URL.Query()
        service := qs.Get("service")
        if service == "" {
            http.Error(w, "missing required query param: service", http.StatusBadRequest)
            return
        }
        endpoint := qs.Get("endpoint") // may be empty
        bucketStr := qs.Get("bucket")
        if bucketStr == "" {
            http.Error(w, "missing required query param: bucket", http.StatusBadRequest)
            return
        }
        start, err := time.Parse(time.RFC3339, bucketStr)
        if err != nil {
            http.Error(w, "invalid bucket timestamp", http.StatusBadRequest)
            return
        }
        // End of bucket = start + 1 minute.
        end := start.Add(time.Minute)
        // Limit handling.
        limit := 20
        if limStr := qs.Get("limit"); limStr != "" {
            if l, err := strconv.Atoi(limStr); err == nil && l > 0 {
                if l > 100 {
                    limit = 100
                } else {
                    limit = l
                }
            }
        }

        var rows pgx.Rows
        var query string
        if endpoint == "" {
            query = `SELECT DISTINCT trace_id FROM spans WHERE service = $1 AND start_ts >= $2::timestamptz AND start_ts < $3::timestamptz ORDER BY trace_id LIMIT $4`
            rows, err = pool.Query(r.Context(), query, service, start, end, limit)
        } else {
            query = `SELECT DISTINCT trace_id FROM spans WHERE service = $1 AND name = $2 AND start_ts >= $3::timestamptz AND start_ts < $4::timestamptz ORDER BY trace_id LIMIT $5`
            rows, err = pool.Query(r.Context(), query, service, endpoint, start, end, limit)
        }
        if err != nil {
            http.Error(w, "query execution error", http.StatusInternalServerError)
            return
        }
        defer rows.Close()

        var ids []string
        for rows.Next() {
            var id string
            if err := rows.Scan(&id); err != nil {
                http.Error(w, "scan error", http.StatusInternalServerError)
                return
            }
            ids = append(ids, id)
        }
        if rows.Err() != nil {
            http.Error(w, "rows error", http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        if err := json.NewEncoder(w).Encode(ids); err != nil {
            http.Error(w, "encoding error", http.StatusInternalServerError)
            return
        }
    }
}

// SpanRow mirrors the spans table columns.
type SpanRow struct {
    TraceID    string  `json:"trace_id"`
    SpanID     string  `json:"span_id"`
    ParentID   *string `json:"parent_id,omitempty"`
    Service    string  `json:"service"`
    Name       string  `json:"name"`
    Kind       *string `json:"kind,omitempty"`
    StartTS    string  `json:"start_ts"` // RFC3339 string
    DurationNS int64   `json:"duration_ns"`
    Status     *string `json:"status,omitempty"`
    Attrs      map[string]interface{} `json:"attrs,omitempty"`
}

// GetTraceSpans returns all spans belonging to a trace ID.
func GetTraceSpans(pool *pgxpool.Pool) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        traceID := chi.URLParam(r, "id") // chi imported below
        if traceID == "" {
            http.Error(w, "missing trace id in path", http.StatusBadRequest)
            return
        }
        rows, err := pool.Query(r.Context(), `SELECT trace_id, span_id, parent_id, service, name, kind, start_ts, duration_ns, status, attrs FROM spans WHERE trace_id = $1 ORDER BY start_ts`, traceID)
        if err != nil {
            http.Error(w, "query execution error", http.StatusInternalServerError)
            return
        }
        defer rows.Close()

        var spans []SpanRow
        for rows.Next() {
            var s SpanRow
            var start time.Time
            if err := rows.Scan(&s.TraceID, &s.SpanID, &s.ParentID, &s.Service, &s.Name, &s.Kind, &start, &s.DurationNS, &s.Status, &s.Attrs); err != nil {
                http.Error(w, "scan error", http.StatusInternalServerError)
                return
            }
            s.StartTS = start.UTC().Format(time.RFC3339)
            spans = append(spans, s)
        }
        if rows.Err() != nil {
            http.Error(w, "rows error", http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        if err := json.NewEncoder(w).Encode(spans); err != nil {
            http.Error(w, "encoding error", http.StatusInternalServerError)
            return
        }
    }
}

// LogRow mirrors the logs table columns.
type LogRow struct {
    TS       string `json:"ts"` // RFC3339 string
    Service  string `json:"service"`
    Severity *string `json:"severity,omitempty"`
    Body     string `json:"body"`
    TraceID  *string `json:"trace_id,omitempty"`
    SpanID   *string `json:"span_id,omitempty"`
    Attrs    map[string]interface{} `json:"attrs,omitempty"`
}

// GetTraceLogs returns log entries associated with a trace ID.
func GetTraceLogs(pool *pgxpool.Pool) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        traceID := chi.URLParam(r, "id")
        if traceID == "" {
            http.Error(w, "missing trace id in path", http.StatusBadRequest)
            return
        }
        rows, err := pool.Query(r.Context(), `SELECT ts, service, severity, body, trace_id, span_id, attrs FROM logs WHERE trace_id = $1 ORDER BY ts`, traceID)
        if err != nil {
            http.Error(w, "query execution error", http.StatusInternalServerError)
            return
        }
        defer rows.Close()

        var logs []LogRow
        for rows.Next() {
            var l LogRow
            var ts time.Time
            if err := rows.Scan(&ts, &l.Service, &l.Severity, &l.Body, &l.TraceID, &l.SpanID, &l.Attrs); err != nil {
                http.Error(w, "scan error", http.StatusInternalServerError)
                return
            }
            l.TS = ts.UTC().Format(time.RFC3339)
            logs = append(logs, l)
        }
        if rows.Err() != nil {
            http.Error(w, "rows error", http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        if err := json.NewEncoder(w).Encode(logs); err != nil {
            http.Error(w, "encoding error", http.StatusInternalServerError)
            return
        }
    }
}
