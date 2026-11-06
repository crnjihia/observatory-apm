// path: api/handlers/latency.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
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
//   offset (optional, default 0)
func GetLatency(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		qs := r.URL.Query()
		service := qs.Get("service")
		if service == "" {
			http.Error(w, `{"error":"missing required query param: service"}`, http.StatusBadRequest)
			return
		}

		startStr := qs.Get("start")
		endStr := qs.Get("end")
		if startStr == "" || endStr == "" {
			http.Error(w, `{"error":"missing required query params: start and end"}`, http.StatusBadRequest)
			return
		}

		start, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			http.Error(w, `{"error":"invalid start timestamp, must be RFC3339 format"}`, http.StatusBadRequest)
			return
		}

		end, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			http.Error(w, `{"error":"invalid end timestamp, must be RFC3339 format"}`, http.StatusBadRequest)
			return
		}

		if end.Before(start) {
			http.Error(w, `{"error":"start timestamp must be before or equal to end timestamp"}`, http.StatusBadRequest)
			return
		}

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

		offset := 0
		if offStr := qs.Get("offset"); offStr != "" {
			if o, err := strconv.Atoi(offStr); err == nil && o >= 0 {
				offset = o
			}
		}

		if pool == nil {
			_ = json.NewEncoder(w).Encode([]LatencyRow{})
			return
		}

		endpoint := qs.Get("endpoint")

		var rows pgx.Rows
		var query string
		if endpoint == "" {
			query = `SELECT bucket, service, endpoint, p50, p95, p99, span_count, error_rate
                     FROM endpoint_latency_agg
                     WHERE service = $1
                       AND bucket >= $2::timestamptz
                       AND bucket <= $3::timestamptz
                     ORDER BY bucket ASC
                     LIMIT $4 OFFSET $5`
			rows, err = pool.Query(r.Context(), query, service, start, end, limit, offset)
		} else {
			query = `SELECT bucket, service, endpoint, p50, p95, p99, span_count, error_rate
                     FROM endpoint_latency_agg
                     WHERE service = $1
                       AND endpoint = $2
                       AND bucket >= $3::timestamptz
                       AND bucket <= $4::timestamptz
                     ORDER BY bucket ASC
                     LIMIT $5 OFFSET $6`
			rows, err = pool.Query(r.Context(), query, service, endpoint, start, end, limit, offset)
		}

		if err != nil {
			http.Error(w, `{"error":"failed to query latency series"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var results []LatencyRow
		for rows.Next() {
			var row LatencyRow
			var bucketTime time.Time
			if err := rows.Scan(
				&bucketTime,
				&row.Service,
				&row.Endpoint,
				&row.P50,
				&row.P95,
				&row.P99,
				&row.SpanCount,
				&row.ErrorRate,
			); err != nil {
				http.Error(w, `{"error":"failed to scan latency row"}`, http.StatusInternalServerError)
				return
			}
			row.Bucket = bucketTime.UTC().Format(time.RFC3339)
			results = append(results, row)
		}

		if results == nil {
			results = []LatencyRow{}
		}

		_ = json.NewEncoder(w).Encode(results)
	}
}
