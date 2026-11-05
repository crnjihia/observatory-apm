// path: C:/Users/crnji/Videos/Projects/Web Development/New/observatory-apm/api/handlers/latency.go
package handlers

import (
    "encoding/json"
    "net/http"
    "strconv"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

// LatencyRow mirrors the columns selected from the continuous aggregate.
type LatencyRow struct {
    Bucket    string  `json:"bucket"`
    Service   string  `json:"service"`
    Endpoint  string  `json:"endpoint"`
    P50       float64 `json:"p50"`
    P95       float64 `json:"p95"`
    P99       float64 `json:"p99"`
    SpanCount int64   `json:"span_count"`
    ErrorRate float64 `json:"error_rate"`
}

// GetLatency returns latency series for a given service/endpoint and time range.
// Expected query parameters:
//   service (required) – service name
//   endpoint (optional) – endpoint name filter
//   start (required) – RFC3339 timestamp (inclusive)
//   end (required) – RFC3339 timestamp (inclusive)
//   limit (optional, default 100, max 1000)
func GetLatency(pool *pgxpool.Pool) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        qs := r.URL.Query()
        service := qs.Get("service")
        if service == "" {
            http.Error(w, "missing required query param: service", http.StatusBadRequest)
            return
        }
        endpoint := qs.Get("endpoint") // optional
        startStr := qs.Get("start")
        endStr := qs.Get("end")
        if startStr == "" || endStr == "" {
            http.Error(w, "missing required query params: start and end", http.StatusBadRequest)
            return
        }
        start, err := time.Parse(time.RFC3339, startStr)
        if err != nil {
            http.Error(w, "invalid start timestamp", http.StatusBadRequest)
            return
        }
        end, err := time.Parse(time.RFC3339, endStr)
        if err != nil {
            http.Error(w, "invalid end timestamp", http.StatusBadRequest)
            return
        }
        // Limit handling – default 100, max 1000.
        limit := 100
        if limStr := qs.Get("limit"); limStr != "" {
            if l, err := strconv.Atoi(limStr); err == nil && l > 0 {
                if l > 1000 {
                    limit = 1000
                } else {
                    limit = l
                }
            }
        }

        var rows pgx.Rows
        var query string
        if endpoint == "" {
            query = `SELECT bucket, service, endpoint, p50, p95, p99, span_count, error_rate
                     FROM endpoint_latency_agg
                     WHERE service = $1
                       AND bucket >= $2::timestamptz
                       AND bucket <= $3::timestamptz
                     ORDER BY bucket ASC
                     LIMIT $4`
            rows, err = pool.Query(r.Context(), query, service, start, end, limit)
        } else {
            query = `SELECT bucket, service, endpoint, p50, p95, p99, span_count, error_rate
                     FROM endpoint_latency_agg
                     WHERE service = $1
                       AND endpoint = $2
                       AND bucket >= $3::timestamptz
                       AND bucket <= $4::timestamptz
                     ORDER BY bucket ASC
                     LIMIT $5`
            rows, err = pool.Query(r.Context(), query, service, endpoint, start, end, limit)
        }
        if err != nil {
            http.Error(w, "query execution error", http.StatusInternalServerError)
            return
        }
        defer rows.Close()

        var data []LatencyRow
        for rows.Next() {
            var lr LatencyRow
            if err := rows.Scan(&lr.Bucket, &lr.Service, &lr.Endpoint, &lr.P50, &lr.P95, &lr.P99, &lr.SpanCount, &lr.ErrorRate); err != nil {
                http.Error(w, "scan error", http.StatusInternalServerError)
                return
            }
            data = append(data, lr)
        }
        if rows.Err() != nil {
            http.Error(w, "rows error", http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        if err := json.NewEncoder(w).Encode(data); err != nil {
            http.Error(w, "encoding error", http.StatusInternalServerError)
            return
        }
    }
}
