// path: api/handlers/heatmap.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HeatmapCell represents an individual (time_bucket, latency_bin) block in the 2D matrix.
type HeatmapCell struct {
	TimeBucket   string  `json:"time_bucket"`
	LatencyMinMs float64 `json:"latency_min_ms"`
	LatencyMaxMs float64 `json:"latency_max_ms"`
	Count        int64   `json:"count"`
	ErrorCount   int64   `json:"error_count"`
}

// PercentileOverlay provides statistical trend line data points across time buckets.
type PercentileOverlay struct {
	TimeBucket string  `json:"time_bucket"`
	P50        float64 `json:"p50"`
	P95        float64 `json:"p95"`
	P99        float64 `json:"p99"`
}

// HeatmapResponse is the complete 2D payload consumed by D3.js Heatmap components.
type HeatmapResponse struct {
	Service       string              `json:"service"`
	Endpoint      string              `json:"endpoint,omitempty"`
	Start         string              `json:"start"`
	End           string              `json:"end"`
	MinDurationMs float64             `json:"min_duration_ms"`
	MaxDurationMs float64             `json:"max_duration_ms"`
	Cells         []HeatmapCell       `json:"cells"`
	Overlays      []PercentileOverlay `json:"overlays"`
}

// GetHeatmapBuckets returns 2D latency distribution matrix for D3 visualization.
// Query parameters:
//   service (required)
//   endpoint (optional)
//   start (required, RFC3339)
//   end (required, RFC3339)
//   bins (optional, default 20)
func GetHeatmapBuckets(pool *pgxpool.Pool) http.HandlerFunc {
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
			http.Error(w, `{"error":"invalid start timestamp"}`, http.StatusBadRequest)
			return
		}

		end, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			http.Error(w, `{"error":"invalid end timestamp"}`, http.StatusBadRequest)
			return
		}

		if end.Before(start) {
			http.Error(w, `{"error":"start timestamp must be before end timestamp"}`, http.StatusBadRequest)
			return
		}

		numBins := 20
		if bStr := qs.Get("bins"); bStr != "" {
			if b, err := strconv.Atoi(bStr); err == nil && b >= 5 && b <= 100 {
				numBins = b
			}
		}

		endpoint := qs.Get("endpoint")

		resp := HeatmapResponse{
			Service:       service,
			Endpoint:      endpoint,
			Start:         start.UTC().Format(time.RFC3339),
			End:           end.UTC().Format(time.RFC3339),
			MinDurationMs: 0,
			MaxDurationMs: 3500.0,
			Cells:         []HeatmapCell{},
			Overlays:      []PercentileOverlay{},
		}

		if pool == nil {
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// 1. Fetch continuous aggregate overlays (P50, P95, P99)
		overlayQuery := `
			SELECT bucket, p50, p95, p99
			FROM endpoint_latency_agg
			WHERE service = $1
			  AND ($2 = '' OR endpoint = $2)
			  AND bucket >= $3::timestamptz
			  AND bucket <= $4::timestamptz
			ORDER BY bucket ASC`

		rows, err := pool.Query(r.Context(), overlayQuery, service, endpoint, start, end)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var bTime time.Time
				var ov PercentileOverlay
				if err := rows.Scan(&bTime, &ov.P50, &ov.P95, &ov.P99); err == nil {
					ov.TimeBucket = bTime.UTC().Format(time.RFC3339)
					resp.Overlays = append(resp.Overlays, ov)
					if ov.P99 > resp.MaxDurationMs {
						resp.MaxDurationMs = ov.P99 * 1.1
					}
				}
			}
		}

		// 2. Compute dynamic latency bins across 1-minute time windows
		binSize := resp.MaxDurationMs / float64(numBins)
		if binSize <= 0 {
			binSize = 50.0
		}

		cellQuery := `
			SELECT
				time_bucket('1 minute', start_ts) AS time_bucket,
				FLOOR((duration_ns / 1000000.0) / $5) * $5 AS latency_bin_min,
				COUNT(*) AS span_count,
				COUNT(*) FILTER (WHERE status = 'STATUS_CODE_ERROR') AS error_count
			FROM spans
			WHERE service = $1
			  AND ($2 = '' OR name = $2)
			  AND start_ts >= $3::timestamptz
			  AND start_ts <= $4::timestamptz
			GROUP BY time_bucket, latency_bin_min
			ORDER BY time_bucket ASC, latency_bin_min ASC`

		cellRows, err := pool.Query(r.Context(), cellQuery, service, endpoint, start, end, binSize)
		if err == nil {
			defer cellRows.Close()
			for cellRows.Next() {
				var bTime time.Time
				var cell HeatmapCell
				if err := cellRows.Scan(&bTime, &cell.LatencyMinMs, &cell.Count, &cell.ErrorCount); err == nil {
					cell.TimeBucket = bTime.UTC().Format(time.RFC3339)
					cell.LatencyMaxMs = cell.LatencyMinMs + binSize
					resp.Cells = append(resp.Cells, cell)
				}
			}
		}

		_ = json.NewEncoder(w).Encode(resp)
	}
}

