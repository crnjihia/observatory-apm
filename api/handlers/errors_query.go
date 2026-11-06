// path: api/handlers/errors_query.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrorItem captures failure context from an individual error span.
type ErrorItem struct {
	TraceID      string    `json:"trace_id"`
	SpanID       string    `json:"span_id"`
	Service      string    `json:"service"`
	Endpoint     string    `json:"endpoint"`
	ErrorMessage string    `json:"error_message"`
	DurationMs   float64   `json:"duration_ms"`
	StartTs      time.Time `json:"start_ts"`
}

// ErrorGroup aggregates error occurrences by service and error signature.
type ErrorGroup struct {
	Service      string `json:"service"`
	Endpoint     string `json:"endpoint"`
	ErrorMessage string `json:"error_message"`
	Count        int64  `json:"count"`
}

// ErrorSummaryResponse provides the high-level error analytics consumed by the dashboard.
type ErrorSummaryResponse struct {
	TotalErrors int64        `json:"total_errors"`
	ErrorGroups []ErrorGroup `json:"error_groups"`
	RecentSpans []ErrorItem  `json:"recent_spans"`
}

// GetErrors returns failing traces, error rates, and grouped root-cause error messages.
// Query parameters:
//   service (optional)
//   limit (optional, default 50)
//   offset (optional, default 0)
func GetErrors(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		qs := r.URL.Query()
		service := qs.Get("service")

		limit := 50
		if limStr := qs.Get("limit"); limStr != "" {
			if l, err := strconv.Atoi(limStr); err == nil && l > 0 {
				if l > 200 {
					limit = 200
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

		resp := ErrorSummaryResponse{
			TotalErrors: 0,
			ErrorGroups: []ErrorGroup{},
			RecentSpans: []ErrorItem{},
		}

		if pool == nil {
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		ctx := r.Context()

		// 1. Query grouped error signatures
		groupQuery := `
			SELECT
				service,
				name AS endpoint,
				COALESCE(attrs->>'error', attrs->>'error.message', 'Unknown Error') AS error_msg,
				COUNT(*) AS count
			FROM spans
			WHERE status = 'STATUS_CODE_ERROR'
			  AND ($1 = '' OR service = $1)
			GROUP BY service, endpoint, error_msg
			ORDER BY count DESC
			LIMIT 10`

		gRows, err := pool.Query(ctx, groupQuery, service)
		if err == nil {
			defer gRows.Close()
			for gRows.Next() {
				var eg ErrorGroup
				if err := gRows.Scan(&eg.Service, &eg.Endpoint, &eg.ErrorMessage, &eg.Count); err == nil {
					resp.ErrorGroups = append(resp.ErrorGroups, eg)
					resp.TotalErrors += eg.Count
				}
			}
		}

		// 2. Query individual recent error spans
		recentQuery := `
			SELECT
				trace_id,
				span_id,
				service,
				name AS endpoint,
				COALESCE(attrs->>'error', attrs->>'error.message', 'Status Error') AS error_msg,
				ROUND((duration_ns / 1000000.0)::numeric, 2)::float8 AS duration_ms,
				start_ts
			FROM spans
			WHERE status = 'STATUS_CODE_ERROR'
			  AND ($1 = '' OR service = $1)
			ORDER BY start_ts DESC
			LIMIT $2 OFFSET $3`

		rRows, err := pool.Query(ctx, recentQuery, service, limit, offset)
		if err == nil {
			defer rRows.Close()
			for rRows.Next() {
				var item ErrorItem
				if err := rRows.Scan(
					&item.TraceID,
					&item.SpanID,
					&item.Service,
					&item.Endpoint,
					&item.ErrorMessage,
					&item.DurationMs,
					&item.StartTs,
				); err == nil {
					resp.RecentSpans = append(resp.RecentSpans, item)
				}
			}
		}

		_ = json.NewEncoder(w).Encode(resp)
	}
}

