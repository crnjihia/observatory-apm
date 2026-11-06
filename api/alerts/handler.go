// path: api/alerts/handler.go
package alerts

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// APIHandler mounts alerting routes onto a Chi router.
func APIHandler(ruleStore *RuleStore, evaluator *Evaluator) http.Handler {
	r := chi.NewRouter()

	r.Get("/rules", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ruleStore.List())
	})

	r.Post("/rules", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var rule Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, `{"error":"invalid rule payload"}`, http.StatusBadRequest)
			return
		}
		if rule.ID == "" || rule.Name == "" {
			http.Error(w, `{"error":"rule id and name required"}`, http.StatusBadRequest)
			return
		}
		ruleStore.Add(rule)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(rule)
	})

	r.Post("/rules/{id}/toggle", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		id := chi.URLParam(r, "id")
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid toggle payload"}`, http.StatusBadRequest)
			return
		}
		if err := ruleStore.Toggle(id, body.Enabled); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "id": id, "enabled": body.Enabled})
	})

	r.Get("/active", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(evaluator.ActiveAlerts())
	})

	r.Get("/history", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(evaluator.AllAlerts())
	})

	r.Post("/{id}/ack", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		id := chi.URLParam(r, "id")
		alert, ok := evaluator.Acknowledge(id)
		if !ok {
			http.Error(w, `{"error":"alert not found or cannot be acknowledged"}`, http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(alert)
	})

	r.Post("/{id}/resolve", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		id := chi.URLParam(r, "id")
		alert, ok := evaluator.Resolve(id)
		if !ok {
			http.Error(w, `{"error":"alert not found or already resolved"}`, http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(alert)
	})

	return r
}

