// path: api/alerts/evaluator.go
package alerts

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ActiveAlert represents a currently open or acknowledged incident alert.
type ActiveAlert struct {
	ID             string     `json:"id"`
	RuleID         string     `json:"rule_id"`
	RuleName       string     `json:"rule_name"`
	Service        string     `json:"service"`
	Endpoint       string     `json:"endpoint"`
	Metric         string     `json:"metric"`
	ObservedValue  float64    `json:"observed_value"`
	Threshold      float64    `json:"threshold"`
	Severity       Severity   `json:"severity"`
	TriggeredAt    time.Time  `json:"triggered_at"`
	State          string     `json:"state"` // "TRIGGERED", "ACKNOWLEDGED", "RESOLVED"
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	LastNotifiedAt time.Time  `json:"last_notified_at"`
	ConsecutiveNominal int    `json:"-"`
}

// StatisticalProfile tracks running EWMA and variance for adaptive anomaly detection.
type StatisticalProfile struct {
	Count    int64
	EWMA     float64
	Variance float64
	Alpha    float64
}

// NewStatisticalProfile initializes a profile with alpha = 0.2.
func NewStatisticalProfile() *StatisticalProfile {
	return &StatisticalProfile{
		Alpha: 0.2,
	}
}

// Update incorporates a new observation into the running EWMA and variance.
func (p *StatisticalProfile) Update(x float64) (zScore float64) {
	p.Count++
	if p.Count == 1 {
		p.EWMA = x
		p.Variance = 1.0
		return 0.0
	}

	diff := x - p.EWMA
	// Update EWMA
	p.EWMA = p.Alpha*x + (1.0-p.Alpha)*p.EWMA

	// Update running variance
	beta := 0.1
	p.Variance = (1.0-beta)*p.Variance + beta*(diff*diff)

	stdDev := math.Sqrt(p.Variance)
	if stdDev < 1e-4 {
		stdDev = 1e-4
	}

	zScore = diff / stdDev
	return zScore
}

// AlertDispatcher interface for sending notifications across channels.
type AlertDispatcher interface {
	DispatchAlert(ctx context.Context, alert *ActiveAlert, rule Rule) error
	DispatchResolution(ctx context.Context, alert *ActiveAlert, rule Rule) error
}

// Evaluator coordinates rules, statistical metrics, active alerts, and suppression.
type Evaluator struct {
	ruleStore  *RuleStore
	dispatcher AlertDispatcher

	mu       sync.RWMutex
	profiles map[string]*StatisticalProfile // key: service:endpoint:metric
	alerts   map[string]*ActiveAlert        // key: alertID
	activeByRule map[string]string          // key: ruleID:service:endpoint -> alertID
}

// NewEvaluator constructs an alert evaluation engine.
func NewEvaluator(ruleStore *RuleStore, dispatcher AlertDispatcher) *Evaluator {
	return &Evaluator{
		ruleStore:    ruleStore,
		dispatcher:   dispatcher,
		profiles:     make(map[string]*StatisticalProfile),
		alerts:       make(map[string]*ActiveAlert),
		activeByRule: make(map[string]string),
	}
}

// Observe evaluates an observed metric value against all matching rules and EWMA baselines.
func (e *Evaluator) Observe(ctx context.Context, service, endpoint, metric string, value float64) []*ActiveAlert {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 1. Update Statistical Profile
	profileKey := fmt.Sprintf("%s:%s:%s", service, endpoint, metric)
	prof, exists := e.profiles[profileKey]
	if !exists {
		prof = NewStatisticalProfile()
		e.profiles[profileKey] = prof
	}
	zScore := prof.Update(value)

	rules := e.ruleStore.List()
	var triggered []*ActiveAlert

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// Match service and endpoint (supports "*" wildcard)
		if rule.Service != "*" && rule.Service != service {
			continue
		}
		if rule.Endpoint != "*" && rule.Endpoint != endpoint {
			continue
		}
		if rule.Metric != metric {
			continue
		}

		breached := e.checkCondition(rule.Condition, value, rule.Threshold)
		ruleInstKey := fmt.Sprintf("%s:%s:%s", rule.ID, service, endpoint)
		activeID, hasActive := e.activeByRule[ruleInstKey]

		if breached {
			now := time.Now().UTC()
			if hasActive {
				// Alert already open: check cooldown suppression
				activeAlert := e.alerts[activeID]
				activeAlert.ObservedValue = value
				activeAlert.ConsecutiveNominal = 0

				cooldown := time.Duration(rule.CooldownSec) * time.Second
				if cooldown <= 0 {
					cooldown = 300 * time.Second
				}

				if time.Since(activeAlert.LastNotifiedAt) >= cooldown {
					activeAlert.LastNotifiedAt = now
					if e.dispatcher != nil {
						go func(a *ActiveAlert, r Rule) {
							_ = e.dispatcher.DispatchAlert(context.Background(), a, r)
						}(activeAlert, rule)
					}
				}
			} else {
				// New alert instance
				alertID := "alt-" + uuid.New().String()[:8]
				alert := &ActiveAlert{
					ID:            alertID,
					RuleID:        rule.ID,
					RuleName:      rule.Name,
					Service:       service,
					Endpoint:      endpoint,
					Metric:        metric,
					ObservedValue: value,
					Threshold:     rule.Threshold,
					Severity:      rule.Severity,
					TriggeredAt:   now,
					State:         "TRIGGERED",
					LastNotifiedAt: now,
					ConsecutiveNominal: 0,
				}

				e.alerts[alertID] = alert
				e.activeByRule[ruleInstKey] = alertID
				triggered = append(triggered, alert)

				log.Printf("[Alerting] TRIGGERED: %s (%s %s) = %.2f (threshold: %.2f, z-score: %.2f)",
					rule.Name, service, endpoint, value, rule.Threshold, zScore)

				if e.dispatcher != nil {
					go func(a *ActiveAlert, r Rule) {
						_ = e.dispatcher.DispatchAlert(context.Background(), a, r)
					}(alert, rule)
				}
			}
		} else if hasActive {
			// Metric returned below threshold: auto-resolution check
			activeAlert := e.alerts[activeID]
			activeAlert.ConsecutiveNominal++

			if activeAlert.ConsecutiveNominal >= 2 {
				// Mark resolved
				now := time.Now().UTC()
				activeAlert.State = "RESOLVED"
				activeAlert.ResolvedAt = &now
				delete(e.activeByRule, ruleInstKey)

				log.Printf("[Alerting] AUTO-RESOLVED: %s (%s %s)", rule.Name, service, endpoint)

				if e.dispatcher != nil {
					go func(a *ActiveAlert, r Rule) {
						_ = e.dispatcher.DispatchResolution(context.Background(), a, r)
					}(activeAlert, rule)
				}
			}
		}
	}

	return triggered
}

func (e *Evaluator) checkCondition(cond Condition, val, threshold float64) bool {
	switch cond {
	case ConditionGT:
		return val > threshold
	case ConditionGTE:
		return val >= threshold
	case ConditionLT:
		return val < threshold
	case ConditionLTE:
		return val <= threshold
	default:
		return val >= threshold
	}
}

// ActiveAlerts returns all currently open or acknowledged alerts.
func (e *Evaluator) ActiveAlerts() []*ActiveAlert {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var active []*ActiveAlert
	for _, a := range e.alerts {
		if a.State != "RESOLVED" {
			cp := *a
			active = append(active, &cp)
		}
	}
	return active
}

// AllAlerts returns complete alert history.
func (e *Evaluator) AllAlerts() []*ActiveAlert {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var all []*ActiveAlert
	for _, a := range e.alerts {
		cp := *a
		all = append(all, &cp)
	}
	return all
}

// Acknowledge marks an alert as acknowledged.
func (e *Evaluator) Acknowledge(id string) (*ActiveAlert, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	alert, ok := e.alerts[id]
	if !ok || alert.State != "TRIGGERED" {
		return nil, false
	}
	now := time.Now().UTC()
	alert.State = "ACKNOWLEDGED"
	alert.AcknowledgedAt = &now
	cp := *alert
	return &cp, true
}

// Resolve marks an alert as manually resolved.
func (e *Evaluator) Resolve(id string) (*ActiveAlert, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	alert, ok := e.alerts[id]
	if !ok || alert.State == "RESOLVED" {
		return nil, false
	}
	now := time.Now().UTC()
	alert.State = "RESOLVED"
	alert.ResolvedAt = &now

	// Clean up activeByRule index
	for k, v := range e.activeByRule {
		if v == id {
			delete(e.activeByRule, k)
			break
		}
	}

	cp := *alert
	return &cp, true
}

