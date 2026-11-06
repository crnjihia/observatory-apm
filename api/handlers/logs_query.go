// path: api/handlers/logs_query.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LogItem represents an individual correlated log record.
type LogItem struct {
	Ts       time.Time              `json:"ts"`
	Service  string                 `json:"service"`
	Severity string                 `json:"severity"`
	Body     string                 `json:"body"`
	TraceID  string                 `json:"trace_id,omitempty"`
	SpanID   string                 `json:"span_id,omitempty"`
	Attrs    map[string]interface{} `json:"attrs"`
}

// GetTraceLogs returns all logs linked to a specific trace ID.
func GetTraceLogs(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		traceID := chi.URLParam(r, "id")
		if traceID == "" {
			http.Error(w, `{"error":"missing trace ID parameter"}`, http.StatusBadRequest)
			return
		}

		if pool == nil {
			_ = json.NewEncoder(w).Encode([]LogItem{})
			return
		}

		query := `
			SELECT
				ts,
				service,
				severity,
				body,
				COALESCE(trace_id, '') AS trace_id,
				COALESCE(span_id, '') AS span_id,
				attrs
			FROM logs
			WHERE trace_id = $1
			ORDER BY ts ASC`

		rows, err := pool.Query(r.Context(), query, traceID)
		if err != nil {
			http.Error(w, `{"error":"failed to query trace logs"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var logs []LogItem
		for rows.Next() {
			var l LogItem
			var attrsRaw []byte
			if err := rows.Scan(
				&l.Ts,
				&l.Service,
				&l.Severity,
				&l.Body,
				&l.TraceID,
				&l.SpanID,
				&attrsRaw,
			); err != nil {
				http.Error(w, `{"error":"failed to scan log row"}`, http.StatusInternalServerError)
				return
			}
			l.Attrs = make(map[string]interface{})
			if len(attrsRaw) > 0 {
				_ = json.Unmarshal(attrsRaw, &l.Attrs)
			}
			logs = append(logs, l)
		}

		if logs == nil {
			logs = []LogItem{}
		}

		_ = json.NewEncoder(w).Encode(logs)
	}
}

// GetLogs queries global log records with service, severity, text, and time filters.
func GetLogs(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		qs := r.URL.Query()
		service := qs.Get("service")
		severity := qs.Get("severity")
		search := qs.Get("search")

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

		if pool == nil {
			_ = json.NewEncoder(w).Encode([]LogItem{})
			return
		}

		query := `
			SELECT
				ts,
				service,
				severity,
				body,
				COALESCE(trace_id, '') AS trace_id,
				COALESCE(span_id, '') AS span_id,
				attrs
			FROM logs
			WHERE ($1 = '' OR service = $1)
			  AND ($2 = '' OR severity = $2)
			  AND ($3 = '' OR body ILIKE '%' || $3 || '%')
			ORDER BY ts DESC
			LIMIT $4 OFFSET $5`

		rows, err := pool.Query(r.Context(), query, service, severity, search, limit, offset)
		if err != nil {
			http.Error(w, `{"error":"failed to query logs"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var logs []LogItem
		for rows.Next() {
			var l LogItem
			var attrsRaw []byte
			if err := rows.Scan(
				&l.Ts,
				&l.Service,
				&l.Severity,
				&l.Body,
				&l.TraceID,
				&l.SpanID,
				&attrsRaw,
			); err != nil {
				http.Error(w, `{"error":"failed to scan log row"}`, http.StatusInternalServerError)
				return
			}
			l.Attrs = make(map[string]interface{})
			if len(attrsRaw) > 0 {
				_ = json.Unmarshal(attrsRaw, &l.Attrs)
			}
			logs = append(logs, l)
		}

		if logs == nil {
			logs = []LogItem{}
		}

		_ = json.NewEncoder(w).Encode(logs)
	}
}

