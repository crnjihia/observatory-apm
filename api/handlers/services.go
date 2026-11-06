// path: api/handlers/services.go
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ServiceDetail provides enriched service telemetry health summary.
type ServiceDetail struct {
	Name       string    `json:"name"`
	SpanCount  int64     `json:"span_count"`
	ErrorCount int64     `json:"error_count"`
	LastSeen   time.Time `json:"last_seen"`
}

// GetServices returns a JSON array of registered services.
// Query parameters:
//   details (optional, bool): if "true", returns array of ServiceDetail objects
func GetServices(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if pool == nil {
			// Mock fallback for environments running before storage initialization
			mockServices := []string{
				"mpesa-callback-service",
				"daraja-b2c-gateway",
				"ussd-menu-session",
				"bank-settlement-engine",
			}
			_ = json.NewEncoder(w).Encode(mockServices)
			return
		}

		ctx := r.Context()
		includeDetails := r.URL.Query().Get("details") == "true"

		if includeDetails {
			query := `
				SELECT
					service,
					COUNT(*) AS span_count,
					COUNT(*) FILTER (WHERE status = 'STATUS_CODE_ERROR') AS error_count,
					COALESCE(MAX(start_ts), NOW()) AS last_seen
				FROM spans
				GROUP BY service
				ORDER BY service ASC`

			rows, err := pool.Query(ctx, query)
			if err != nil {
				http.Error(w, `{"error":"failed to query service details"}`, http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			var details []ServiceDetail
			for rows.Next() {
				var d ServiceDetail
				if err := rows.Scan(&d.Name, &d.SpanCount, &d.ErrorCount, &d.LastSeen); err != nil {
					http.Error(w, `{"error":"failed to scan service row"}`, http.StatusInternalServerError)
					return
				}
				details = append(details, d)
			}

			if details == nil {
				details = []ServiceDetail{}
			}
			_ = json.NewEncoder(w).Encode(details)
			return
		}

		query := `SELECT DISTINCT service FROM spans ORDER BY service ASC`
		rows, err := pool.Query(ctx, query)
		if err != nil {
			http.Error(w, `{"error":"failed to query services"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var services []string
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				http.Error(w, `{"error":"failed to scan service"}`, http.StatusInternalServerError)
				return
			}
			services = append(services, s)
		}

		if services == nil {
			services = []string{}
		}
		_ = json.NewEncoder(w).Encode(services)
	}
}
