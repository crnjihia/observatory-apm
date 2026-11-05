// path: C:/Users/crnji/Videos/Projects/Web Development/New/observatory-apm/api/handlers/services.go
package handlers

import (
    "encoding/json"
    "net/http"
    "github.com/jackc/pgx/v5/pgxpool"
)

// GetServices returns a JSON array of distinct service names from the spans table.
func GetServices(pool *pgxpool.Pool) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        rows, err := pool.Query(r.Context(), "SELECT DISTINCT service FROM spans ORDER BY service")
        if err != nil {
            http.Error(w, "failed to query services", http.StatusInternalServerError)
            return
        }
        defer rows.Close()
        var services []string
        for rows.Next() {
            var s string
            if err := rows.Scan(&s); err != nil {
                http.Error(w, "scan error", http.StatusInternalServerError)
                return
            }
            services = append(services, s)
        }
        if rows.Err() != nil {
            http.Error(w, "rows error", http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        if err := json.NewEncoder(w).Encode(services); err != nil {
            http.Error(w, "encode error", http.StatusInternalServerError)
        }
    }
}
