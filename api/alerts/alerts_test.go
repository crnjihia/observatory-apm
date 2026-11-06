// path: api/alerts/alerts_test.go
package alerts

import (
	"context"
	"testing"
)

func TestNormalizeKenyanPhone(t *testing.T) {
	cases := []struct {
		input    string
		expected string
		isErr    bool
	}{
		{"0712345678", "+254712345678", false},
		{"0722000000", "+254722000000", false},
		{"0110123456", "+254110123456", false},
		{"254712345678", "+254712345678", false},
		{"+254712345678", "+254712345678", false},
		{"0712 345 678", "+254712345678", false},
		{"(0712) 345-678", "+254712345678", false},
		{"12345", "", true},
		{"0812345678", "", true},
		{"+12025550199", "", true},
	}

	for _, tc := range cases {
		out, err := NormalizeKenyanPhone(tc.input)
		if tc.isErr {
			if err == nil {
				t.Errorf("expected error for input %s, got %s", tc.input, out)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error for input %s: %v", tc.input, err)
			}
			if out != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, out)
			}
		}
	}
}

func TestStatisticalProfileEWMA(t *testing.T) {
	prof := NewStatisticalProfile()

	// Initial values around baseline 50ms
	for i := 0; i < 20; i++ {
		prof.Update(50.0)
	}

	if prof.EWMA < 49.0 || prof.EWMA > 51.0 {
		t.Fatalf("expected EWMA ~50.0, got %.2f", prof.EWMA)
	}

	// Sudden spike to 1800ms
	z := prof.Update(1800.0)
	if z <= 3.0 {
		t.Errorf("expected high Z-score anomaly (>3.0) for 1800ms spike, got %.2f", z)
	}
}

type mockDispatcher struct {
	dispatchedAlerts []*ActiveAlert
	resolutions      []*ActiveAlert
}

func (m *mockDispatcher) DispatchAlert(_ context.Context, alert *ActiveAlert, _ Rule) error {
	m.dispatchedAlerts = append(m.dispatchedAlerts, alert)
	return nil
}

func (m *mockDispatcher) DispatchResolution(_ context.Context, alert *ActiveAlert, _ Rule) error {
	m.resolutions = append(m.resolutions, alert)
	return nil
}

func TestEvaluator_BreachAndSuppression(t *testing.T) {
	ctx := context.Background()
	ruleStore := NewRuleStore()
	disp := &mockDispatcher{}
	eval := NewEvaluator(ruleStore, disp)

	service := "mpesa-payment-gateway"
	endpoint := "/v1/callbacks/mpesa"
	metric := "p99_latency"

	// 1. Observe normal latency (400ms < 1500ms threshold)
	eval.Observe(ctx, service, endpoint, metric, 400.0)
	if len(eval.ActiveAlerts()) != 0 {
		t.Fatalf("expected 0 active alerts for nominal latency")
	}

	// 2. Observe SLA Breach (2200ms >= 1500ms threshold)
	triggered := eval.Observe(ctx, service, endpoint, metric, 2200.0)
	if len(triggered) != 1 {
		t.Fatalf("expected 1 triggered alert, got %d", len(triggered))
	}
	if len(eval.ActiveAlerts()) != 1 {
		t.Fatalf("expected 1 active alert in store, got %d", len(eval.ActiveAlerts()))
	}
	alertID := triggered[0].ID

	// 3. Repeat breach within cooldown (2400ms) -> should be suppressed
	eval.Observe(ctx, service, endpoint, metric, 2400.0)
	if len(eval.ActiveAlerts()) != 1 {
		t.Fatalf("expected alert count to remain 1 (suppressed)")
	}

	// 4. Acknowledge alert
	ack, ok := eval.Acknowledge(alertID)
	if !ok || ack.State != "ACKNOWLEDGED" {
		t.Fatalf("failed to acknowledge alert: %v", ack)
	}

	// 5. Metric returns below threshold twice -> Auto-resolution
	eval.Observe(ctx, service, endpoint, metric, 350.0)
	eval.Observe(ctx, service, endpoint, metric, 320.0)

	if len(eval.ActiveAlerts()) != 0 {
		t.Fatalf("expected alert to be auto-resolved, active count = %d", len(eval.ActiveAlerts()))
	}
}

func TestAfricasTalkingMockDispatch(t *testing.T) {
	ctx := context.Background()
	at := NewAfricasTalkingNotifier()
	at.useMock = true

	err := at.SendSMS(ctx, "+254712345678", "Observatory ALERT: Daraja P99 Latency 2200ms")
	if err != nil {
		t.Fatalf("unexpected SMS dispatch error: %v", err)
	}

	hist := at.History()
	if len(hist) != 1 {
		t.Fatalf("expected 1 recorded SMS, got %d", len(hist))
	}
	if hist[0].Recipient != "+254712345678" {
		t.Fatalf("expected recipient +254712345678, got %s", hist[0].Recipient)
	}
}

func TestSlackWebhookMockDispatch(t *testing.T) {
	ctx := context.Background()
	slack := NewSlackNotifier()
	slack.useMock = true

	rule := Rule{
		Name:      "Test Latency Spike",
		Service:   "mpesa-payment-gateway",
		Endpoint:  "/v1/callbacks/mpesa",
		Metric:    "p99_latency",
		Threshold: 1500.0,
		Severity:  SeverityCritical,
	}

	err := slack.SendAlert(ctx, rule, 2150.0, "Latency exceeded 1500ms SLA", "inc-test-01")
	if err != nil {
		t.Fatalf("unexpected slack send error: %v", err)
	}

	hist := slack.History()
	if len(hist) != 1 {
		t.Fatalf("expected 1 recorded slack alert, got %d", len(hist))
	}
	if len(hist[0].Payload.Blocks) != 3 {
		t.Fatalf("expected 3 Block Kit blocks, got %d", len(hist[0].Payload.Blocks))
	}
}

