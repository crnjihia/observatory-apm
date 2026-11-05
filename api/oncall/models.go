// path: C:/Users/crnji/Videos/Projects/Web Development/New/observatory-apm/api/oncall/models.go
package oncall

import "time"

// IncidentState represents the lifecycle of an incident.
type IncidentState string

const (
    StateTriggered   IncidentState = "triggered"
    StateAcknowledged IncidentState = "acknowledged"
    StateResolved    IncidentState = "resolved"
)

// Incident holds the details of a simulated incident.
type Incident struct {
    ID            string        `json:"id"`
    Service       string        `json:"service"`
    Endpoint      string        `json:"endpoint"`
    Description   string        `json:"description"`
    CreatedAt     time.Time     `json:"created_at"`
    State         IncidentState `json:"state"`
    AcknowledgedAt *time.Time   `json:"acknowledged_at,omitempty"`
    ResolvedAt    *time.Time   `json:"resolved_at,omitempty"`
    // internal tracking for escalation
    escalationIdx int
}

// OnCallSchedule holds the rotating on‑call users.
type OnCallSchedule struct {
    Users []string // phone numbers in Kenyan format, e.g. +2547XXXXXXXX
    // RotationPeriod defines how long each user stays on‑call (seconds).
    RotationPeriod time.Duration
}

// EscalationChain defines the ordered list of users to notify if an incident is not acknowledged.
type EscalationChain struct {
    Users []string // phone numbers or identifiers
    // Timeout before escalating to the next user.
    Timeout time.Duration
}
