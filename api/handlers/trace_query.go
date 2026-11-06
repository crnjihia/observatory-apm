// path: api/handlers/trace_query.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TraceSummary represents top-level metadata for an individual trace.
type TraceSummary struct {
	TraceID         string    `json:"trace_id"`
	Service         string    `json:"service"`
	RootEndpoint    string    `json:"root_endpoint"`
	StartTs         time.Time `json:"start_ts"`
	DurationMs      float64   `json:"duration_ms"`
	SpanCount       int64     `json:"span_count"`
	HasError        bool      `json:"has_error"`
}

// SpanDetail represents an individual span in a trace tree hierarchy.
type SpanDetail struct {
	TraceID    string                 `json:"trace_id"`
	SpanID     string                 `json:"span_id"`
	ParentID   string                 `json:"parent_id,omitempty"`
	Service    string                 `json:"service"`
	Name       string                 `json:"name"`
	Kind       string                 `json:"kind"`
	StartTs    time.Time              `json:"start_ts"`
	DurationNs int64                  `json:"duration_ns"`
	DurationMs float64                `json:"duration_ms"`
	Status     string                 `json:"status"`
	Attrs      map[string]interface{} `json:"attrs"`
}

// TraceWaterfallResponse groups the full waterfall timeline for a trace.
type TraceWaterfallResponse struct {
	TraceID         string       `json:"trace_id"`
	TotalSpans      int          `json:"total_spans"`
	TotalDurationMs float64      `json:"total_duration_ms"`
	HasError        bool         `json:"has_error"`
	Spans           []SpanDetail `json:"spans"`
}

// GetTracesByBucket queries traces within a minute bucket or time range.
// Query parameters:
//   service (required)
//   endpoint (optional)
//   bucket (optional, minute ISO timestamp)
//   start, end (optional RFC3339 range if bucket not specified)
//   limit (default 20, max 100)
//   offset (default 0)
func GetTracesByBucket(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		qs := r.URL.Query()
		service := qs.Get("service")
		if service == "" {
			http.Error(w, `{"error":"missing required query param: service"}`, http.StatusBadRequest)
			return
		}

		var start, end time.Time
		if bucketStr := qs.Get("bucket"); bucketStr != "" {
			t, err := time.Parse(time.RFC3339, bucketStr)
			if err != nil {
				http.Error(w, `{"error":"invalid bucket timestamp"}`, http.StatusBadRequest)
				return
			}
			start = t
			end = t.Add(time.Minute)
		} else {
			startStr := qs.Get("start")
			endStr := qs.Get("end")
			if startStr != "" && endStr != "" {
				s, errS := time.Parse(time.RFC3339, startStr)
				e, errE := time.Parse(time.RFC3339, endStr)
				if errS != nil || errE != nil || e.Before(s) {
					http.Error(w, `{"error":"invalid start/end timestamps"}`, http.StatusBadRequest)
					return
				}
				start = s
				end = e
			} else {
				// Default to past 1 hour
				end = time.Now().UTC()
				start = end.Add(-1 * time.Hour)
			}
		}

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

		offset := 0
		if offStr := qs.Get("offset"); offStr != "" {
			if o, err := strconv.Atoi(offStr); err == nil && o >= 0 {
				offset = o
			}
		}

		if pool == nil {
			_ = json.NewEncoder(w).Encode([]TraceSummary{})
			return
		}

		endpoint := qs.Get("endpoint")

		query := `
			SELECT
				trace_id,
				service,
				name AS root_endpoint,
				start_ts,
				ROUND((duration_ns / 1000000.0)::numeric, 2)::float8 AS duration_ms,
				1 AS span_count,
				(status = 'STATUS_CODE_ERROR') AS has_error
			FROM spans
			WHERE service = $1
			  AND ($2 = '' OR name = $2)
			  AND start_ts >= $3::timestamptz
			  AND start_ts <= $4::timestamptz
			  AND parent_id IS NULL
			ORDER BY start_ts DESC
			LIMIT $5 OFFSET $6`

		rows, err := pool.Query(r.Context(), query, service, endpoint, start, end, limit, offset)
		if err != nil {
			// Fallback without parent_id filter if root span is not explicitly parent_id IS NULL
			fallbackQuery := `
				SELECT
					trace_id,
					service,
					name AS root_endpoint,
					MIN(start_ts) AS start_ts,
					ROUND((MAX(duration_ns) / 1000000.0)::numeric, 2)::float8 AS duration_ms,
					COUNT(*) AS span_count,
					BOOL_OR(status = 'STATUS_CODE_ERROR') AS has_error
				FROM spans
				WHERE service = $1
				  AND ($2 = '' OR name = $2)
				  AND start_ts >= $3::timestamptz
				  AND start_ts <= $4::timestamptz
				GROUP BY trace_id, service, name
				ORDER BY start_ts DESC
				LIMIT $5 OFFSET $6`
			rows, err = pool.Query(r.Context(), fallbackQuery, service, endpoint, start, end, limit, offset)
			if err != nil {
				http.Error(w, `{"error":"failed to query traces"}`, http.StatusInternalServerError)
				return
			}
		}
		defer rows.Close()

		var summaries []TraceSummary
		for rows.Next() {
			var s TraceSummary
			if err := rows.Scan(
				&s.TraceID,
				&s.Service,
				&s.RootEndpoint,
				&s.StartTs,
				&s.DurationMs,
				&s.SpanCount,
				&s.HasError,
			); err != nil {
				http.Error(w, `{"error":"failed to scan trace summary"}`, http.StatusInternalServerError)
				return
			}
			summaries = append(summaries, s)
		}

		if summaries == nil {
			summaries = []TraceSummary{}
		}

		_ = json.NewEncoder(w).Encode(summaries)
	}
}

// GetTraceSpans returns all spans belonging to a trace ID for waterfall rendering.
func GetTraceSpans(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		traceID := chi.URLParam(r, "id")
		if traceID == "" {
			http.Error(w, `{"error":"missing trace ID parameter"}`, http.StatusBadRequest)
			return
		}

		if pool == nil {
			http.Error(w, `{"error":"database unavailable"}`, http.StatusServiceUnavailable)
			return
		}

		query := `
			SELECT
				trace_id,
				span_id,
				COALESCE(parent_id, '') AS parent_id,
				service,
				name,
				kind,
				start_ts,
				duration_ns,
				status,
				attrs
			FROM spans
			WHERE trace_id = $1
			ORDER BY start_ts ASC`

		rows, err := pool.Query(r.Context(), query, traceID)
		if err != nil {
			http.Error(w, `{"error":"failed to query trace spans"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var spans []SpanDetail
		var hasError bool
		var maxDurationNs int64

		for rows.Next() {
			var s SpanDetail
			var attrsRaw []byte
			if err := rows.Scan(
				&s.TraceID,
				&s.SpanID,
				&s.ParentID,
				&s.Service,
				&s.Name,
				&s.Kind,
				&s.StartTs,
				&s.DurationNs,
				&s.Status,
				&attrsRaw,
			); err != nil {
				http.Error(w, `{"error":"failed to scan span"}`, http.StatusInternalServerError)
				return
			}

			s.DurationMs = float64(s.DurationNs) / 1000000.0
			if s.DurationNs > maxDurationNs {
				maxDurationNs = s.DurationNs
			}
			if s.Status == "STATUS_CODE_ERROR" {
				hasError = true
			}

			s.Attrs = make(map[string]interface{})
			if len(attrsRaw) > 0 {
				_ = json.Unmarshal(attrsRaw, &s.Attrs)
			}

			spans = append(spans, s)
		}

		if len(spans) == 0 {
			http.Error(w, `{"error":"trace not found"}`, http.StatusNotFound)
			return
		}

		response := TraceWaterfallResponse{
			TraceID:         traceID,
			TotalSpans:      len(spans),
			TotalDurationMs: float64(maxDurationNs) / 1000000.0,
			HasError:        hasError,
			Spans:           spans,
		}

		_ = json.NewEncoder(w).Encode(response)
	}
}
