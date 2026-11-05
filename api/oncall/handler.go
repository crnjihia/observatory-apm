package oncall

import (
    "encoding/json"
    "net/http"
    "time"
    "github.com/go-chi/chi/v5"
)

// APIHandler returns a chi Router with the on‑call endpoints.
func APIHandler(store *Store) http.Handler {
    r := chi.NewRouter()
    r.Get("/current", getCurrentOnCall(store))
    r.Get("/incidents", listIncidents(store))
    r.Post("/incidents", createIncident(store))
    r.Post("/incidents/{id}/ack", acknowledgeIncident(store))
    r.Post("/incidents/{id}/resolve", resolveIncident(store))
    return r
}

// getCurrentOnCall returns the current on‑call phone number.
func getCurrentOnCall(store *Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        number := store.CurrentOnCall()
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"on_call": number})
    }
}

// listIncidents returns all incidents.
func listIncidents(store *Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        incidents := store.ListIncidents()
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(incidents)
    }
}

// createIncident creates a new incident from JSON payload.
// Expected fields: service, endpoint, description.
func createIncident(store *Store) http.HandlerFunc {
    type payload struct {
        Service     string `json:"service"`
        Endpoint    string `json:"endpoint"`
        Description string `json:"description"`
    }
    return func(w http.ResponseWriter, r *http.Request) {
        var p payload
        if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
            http.Error(w, "invalid JSON", http.StatusBadRequest)
            return
        }
        inc := store.AddIncident(p.Service, p.Endpoint, p.Description)
        // Kick off escalation timer in background.
        go scheduleEscalation(store, inc)
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(inc)
    }
}

// acknowledgeIncident marks an incident as acknowledged.
func acknowledgeIncident(store *Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        id := chi.URLParam(r, "id")
        inc, ok := store.Acknowledge(id)
        if !ok {
            http.Error(w, "cannot acknowledge", http.StatusBadRequest)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(inc)
    }
}

// resolveIncident marks an incident as resolved.
func resolveIncident(store *Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        id := chi.URLParam(r, "id")
        inc, ok := store.Resolve(id)
        if !ok {
            http.Error(w, "cannot resolve", http.StatusBadRequest)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(inc)
    }
}

// scheduleEscalation runs a timer that, after the configured timeout, notifies the next escalation user if the incident is still pending.
func scheduleEscalation(store *Store, inc *Incident) {
    timeout := store.escalation.Timeout
    if timeout == 0 {
        timeout = 5 * time.Minute // default escalation after 5 min
    }
    timer := time.NewTimer(timeout)
    <-timer.C
    // Re‑fetch latest incident state.
    cur := store.GetIncident(inc.ID)
    if cur == nil || cur.State != StateTriggered {
        return // already acked/resolved
    }
    // Increment escalation index and notify.
    store.mu.Lock()
    cur.escalationIdx++
    target := store.NextEscalationTarget(cur)
    store.mu.Unlock()
    // Placeholder: log the escalation target. In production you would call the alert notifier.
    if target != "" {
        // fmt.Printf("[Escalation] Notifying %s about incident %s\n", target, inc.ID)
    }
}
