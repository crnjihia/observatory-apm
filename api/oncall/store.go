// path: C:/Users/crnji/Videos/Projects/Web Development/New/observatory-apm/api/oncall/store.go
package oncall

import (
    "encoding/json"
    "os"
    "sync"
    "time"
    "github.com/google/uuid"
)

// Store holds incidents and schedule state in memory (protected by a mutex).
type Store struct {
    mu        sync.Mutex
    incidents map[string]*Incident
    schedule  OnCallSchedule
    escalation EscalationChain
}

// NewStore creates a Store with the given schedule and escalation configuration.
func NewStore(schedule OnCallSchedule, escalation EscalationChain) *Store {
    return &Store{
        incidents: make(map[string]*Incident),
        schedule:   schedule,
        escalation: escalation,
    }
}

// AddIncident creates a new incident, stores it, and returns it.
func (s *Store) AddIncident(service, endpoint, description string) *Incident {
    s.mu.Lock()
    defer s.mu.Unlock()
    id := uuid.New().String()
    inc := &Incident{
        ID:          id,
        Service:     service,
        Endpoint:    endpoint,
        Description: description,
        CreatedAt:   time.Now().UTC(),
        State:       StateTriggered,
        escalationIdx: 0,
    }
    s.incidents[id] = inc
    return inc
}

// GetIncident returns a copy of the incident (or nil if not found).
func (s *Store) GetIncident(id string) *Incident {
    s.mu.Lock()
    defer s.mu.Unlock()
    if inc, ok := s.incidents[id]; ok {
        // Return a shallow copy to avoid callers mutating internal state.
        copy := *inc
        return &copy
    }
    return nil
}

// ListIncidents returns a slice of all incidents (shallow copies).
func (s *Store) ListIncidents() []*Incident {
    s.mu.Lock()
    defer s.mu.Unlock()
    out := make([]*Incident, 0, len(s.incidents))
    for _, inc := range s.incidents {
        cp := *inc
        out = append(out, &cp)
    }
    return out
}

// Acknowledge marks the incident as acknowledged.
func (s *Store) Acknowledge(id string) (*Incident, bool) {
    s.mu.Lock()
    defer s.mu.Unlock()
    inc, ok := s.incidents[id]
    if !ok || inc.State != StateTriggered {
        return nil, false
    }
    now := time.Now().UTC()
    inc.State = StateAcknowledged
    inc.AcknowledgedAt = &now
    return inc, true
}

// Resolve marks the incident as resolved.
func (s *Store) Resolve(id string) (*Incident, bool) {
    s.mu.Lock()
    defer s.mu.Unlock()
    inc, ok := s.incidents[id]
    if !ok || inc.State != StateAcknowledged {
        return nil, false
    }
    now := time.Now().UTC()
    inc.State = StateResolved
    inc.ResolvedAt = &now
    return inc, true
}

// CurrentOnCall returns the phone number of the person currently on‑call.
func (s *Store) CurrentOnCall() string {
    // Rotation based on elapsed time since Unix epoch.
    if len(s.schedule.Users) == 0 {
        return ""
    }
    period := s.schedule.RotationPeriod
    if period == 0 {
        period = 24 * time.Hour // default 24h per person
    }
    now := time.Now().UTC()
    elapsed := now.Sub(time.Unix(0, 0))
    idx := int(elapsed / period) % len(s.schedule.Users)
    return s.schedule.Users[idx]
}

// NextEscalationTarget returns the next user to notify for a given incident.
func (s *Store) NextEscalationTarget(inc *Incident) string {
    if len(s.escalation.Users) == 0 {
        // fallback to current on‑call
        return s.CurrentOnCall()
    }
    idx := inc.escalationIdx
    if idx >= len(s.escalation.Users) {
        // wrap around to on‑call if we exhausted the chain
        return s.CurrentOnCall()
    }
    return s.escalation.Users[idx]
}

// Persist saves incidents to a JSON file (called on graceful shutdown).
func (s *Store) Persist(path string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    data := make([]*Incident, 0, len(s.incidents))
    for _, inc := range s.incidents {
        data = append(data, inc)
    }
    f, err := os.Create(path)
    if err != nil {
        return err
    }
    defer f.Close()
    enc := json.NewEncoder(f)
    enc.SetIndent("", "  ")
    return enc.Encode(data)
}

// Load restores incidents from a JSON file (if it exists).
func (s *Store) Load(path string) error {
    f, err := os.Open(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil // nothing to load – start fresh
        }
        return err
    }
    defer f.Close()
    var data []*Incident
    dec := json.NewDecoder(f)
    if err := dec.Decode(&data); err != nil {
        return err
    }
    s.mu.Lock()
    defer s.mu.Unlock()
    for _, inc := range data {
        s.incidents[inc.ID] = inc
    }
    return nil
}
