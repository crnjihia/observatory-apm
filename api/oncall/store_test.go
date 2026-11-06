// path: api/oncall/store_test.go
package oncall

import (
	"testing"
	"time"
)

func TestOnCallStoreOperations(t *testing.T) {
	schedule := OnCallSchedule{
		Users:          []string{"+254711000001", "+254711000002"},
		RotationPeriod: 24 * time.Hour,
	}
	escalation := EscalationChain{
		Users:   []string{"+254711000099"},
		Timeout: 5 * time.Minute,
	}

	store := NewStore(schedule, escalation)

	// 1. Current on-call engineer verification
	current := store.CurrentOnCall()
	if current != "+254711000001" && current != "+254711000002" {
		t.Fatalf("expected valid Kenyan on-call number from schedule, got %s", current)
	}

	// 2. Add incident
	inc := store.AddIncident("mpesa-callback-service", "/v1/callbacks/mpesa", "Simulated Daraja callback latency spike > 2000ms")
	if inc == nil || inc.ID == "" {
		t.Fatal("expected created incident with valid UUID")
	}
	if inc.State != StateTriggered {
		t.Fatalf("expected state %s, got %s", StateTriggered, inc.State)
	}

	// 3. Retrieve incident
	fetched := store.GetIncident(inc.ID)
	if fetched == nil || fetched.ID != inc.ID {
		t.Fatalf("failed to retrieve incident by id %s", inc.ID)
	}

	// 4. Acknowledge incident
	_, acked := store.Acknowledge(inc.ID)
	if !acked {
		t.Fatal("failed to acknowledge incident")
	}
	fetched = store.GetIncident(inc.ID)
	if fetched.State != StateAcknowledged {
		t.Fatalf("expected state %s, got %s", StateAcknowledged, fetched.State)
	}
	if fetched.AcknowledgedAt == nil {
		t.Fatal("expected AcknowledgedAt timestamp to be set")
	}

	// 5. Resolve incident
	_, resolved := store.Resolve(inc.ID)
	if !resolved {
		t.Fatal("failed to resolve incident")
	}
	fetched = store.GetIncident(inc.ID)
	if fetched.State != StateResolved {
		t.Fatalf("expected state %s, got %s", StateResolved, fetched.State)
	}
	if fetched.ResolvedAt == nil {
		t.Fatal("expected ResolvedAt timestamp to be set")
	}

	// 6. List incidents
	all := store.ListIncidents()
	if len(all) != 1 {
		t.Fatalf("expected 1 incident, found %d", len(all))
	}
}
