// path: api/alerts/rules.go
package alerts

import (
	"fmt"
	"sync"
)

// Severity defines the alert severity level.
type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityWarning  Severity = "WARNING"
	SeverityInfo     Severity = "INFO"
)

// Condition defines the comparison operator.
type Condition string

const (
	ConditionGT  Condition = "gt"
	ConditionGTE Condition = "gte"
	ConditionLT  Condition = "lt"
	ConditionLTE Condition = "lte"
)

// Rule represents an alerting rule definition.
type Rule struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Service     string    `json:"service"`  // "*" for all services
	Endpoint    string    `json:"endpoint"` // "*" for all endpoints
	Metric      string    `json:"metric"`   // "p95_latency", "p99_latency", "error_rate"
	Condition   Condition `json:"condition"`
	Threshold   float64   `json:"threshold"`
	WindowSec   int       `json:"window_sec"`
	Severity    Severity  `json:"severity"`
	Channels    []string  `json:"channels"` // "slack", "sms"
	Enabled     bool      `json:"enabled"`
	CooldownSec int       `json:"cooldown_sec"` // Suppression period to prevent alert storms
}

// RuleStore provides concurrent management of alerting rules.
type RuleStore struct {
	mu    sync.RWMutex
	rules map[string]Rule
}

// NewRuleStore constructs a RuleStore preloaded with standard Kenyan fintech SLA rules.
func NewRuleStore() *RuleStore {
	store := &RuleStore{
		rules: make(map[string]Rule),
	}

	// 1. Safaricom Daraja P99 Latency SLA Rule (1500ms timeout boundary)
	store.Add(Rule{
		ID:          "rule-daraja-sla-p99",
		Name:        "Safaricom Daraja P99 Callback Latency SLA Breach",
		Service:     "mpesa-payment-gateway",
		Endpoint:    "/v1/callbacks/mpesa",
		Metric:      "p99_latency",
		Condition:   ConditionGTE,
		Threshold:   1500.0, // 1500 ms SLA boundary
		WindowSec:   60,
		Severity:    SeverityCritical,
		Channels:    []string{"sms", "slack"},
		Enabled:     true,
		CooldownSec: 300, // 5 min cooldown
	})

	// 2. High Error Rate Anomaly (Error rate >= 2.0%)
	store.Add(Rule{
		ID:          "rule-high-error-rate",
		Name:        "Payment Gateway Error Rate Surge",
		Service:     "*",
		Endpoint:    "*",
		Metric:      "error_rate",
		Condition:   ConditionGTE,
		Threshold:   0.02, // 2% error rate
		WindowSec:   60,
		Severity:    SeverityCritical,
		Channels:    []string{"sms", "slack"},
		Enabled:     true,
		CooldownSec: 300,
	})

	// 3. USSD Session Latency Warning (P95 >= 800ms)
	store.Add(Rule{
		ID:          "rule-ussd-latency-warn",
		Name:        "USSD Telco Menu Response Latency Warning",
		Service:     "ussd-banking-session",
		Endpoint:    "/ussd/v2/menu",
		Metric:      "p95_latency",
		Condition:   ConditionGTE,
		Threshold:   800.0,
		WindowSec:   120,
		Severity:    SeverityWarning,
		Channels:    []string{"slack"},
		Enabled:     true,
		CooldownSec: 600, // 10 min cooldown
	})

	return store
}

// Add inserts or updates an alert rule.
func (s *RuleStore) Add(rule Rule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules[rule.ID] = rule
}

// Get retrieves a rule by ID.
func (s *RuleStore) Get(id string) (Rule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.rules[id]
	return r, ok
}

// List returns all registered rules.
func (s *RuleStore) List() []Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]Rule, 0, len(s.rules))
	for _, r := range s.rules {
		list = append(list, r)
	}
	return list
}

// Toggle flips the enabled state of a rule.
func (s *RuleStore) Toggle(id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rules[id]
	if !ok {
		return fmt.Errorf("rule not found: %s", id)
	}
	r.Enabled = enabled
	s.rules[id] = r
	return nil
}
