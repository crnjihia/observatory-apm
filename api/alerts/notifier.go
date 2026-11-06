// path: api/alerts/notifier.go
package alerts

import (
	"context"
	"fmt"
	"log"
	"strings"

	"observatory-apm/api/oncall"
)

// ChannelNotifier coordinates multi-channel delivery across SMS and Slack.
type ChannelNotifier struct {
	atNotifier    *AfricasTalkingNotifier
	slackNotifier *SlackNotifier
	oncallStore   *oncall.Store
}

// NewChannelNotifier constructs a coordinated dispatcher.
func NewChannelNotifier(
	at *AfricasTalkingNotifier,
	slack *SlackNotifier,
	oncallStore *oncall.Store,
) *ChannelNotifier {
	return &ChannelNotifier{
		atNotifier:    at,
		slackNotifier: slack,
		oncallStore:   oncallStore,
	}
}

// DispatchAlert sends incident notifications across all channels enabled for the rule.
func (cn *ChannelNotifier) DispatchAlert(ctx context.Context, alert *ActiveAlert, rule Rule) error {
	description := fmt.Sprintf(
		"ALERT: %s on service '%s' (endpoint: %s). Metric %s observed at %.2f (Threshold: %.2f)",
		rule.Name,
		alert.Service,
		alert.Endpoint,
		alert.Metric,
		alert.ObservedValue,
		alert.Threshold,
	)

	// Automatically mirror into on-call incident store if available
	if cn.oncallStore != nil {
		cn.oncallStore.AddIncident(alert.Service, alert.Endpoint, description)
	}

	for _, ch := range rule.Channels {
		switch strings.ToLower(ch) {
		case "sms":
			if cn.atNotifier != nil && cn.oncallStore != nil {
				recipient := cn.oncallStore.CurrentOnCall()
				if recipient != "" {
					smsText := fmt.Sprintf("Observatory %s: %s. %s is %.1f (Limit: %.1f). Action required: ACK via APM.",
						rule.Severity, rule.Name, alert.Metric, alert.ObservedValue, alert.Threshold)
					if err := cn.atNotifier.SendSMS(ctx, recipient, smsText); err != nil {
						log.Printf("[Notifier] SMS delivery failure to %s: %v", recipient, err)
					}
				}
			}
		case "slack":
			if cn.slackNotifier != nil {
				if err := cn.slackNotifier.SendAlert(ctx, rule, alert.ObservedValue, description, alert.ID); err != nil {
					log.Printf("[Notifier] Slack delivery failure: %v", err)
				}
			}
		}
	}

	return nil
}

// DispatchResolution sends recovery notifications when a metric returns to nominal levels.
func (cn *ChannelNotifier) DispatchResolution(ctx context.Context, alert *ActiveAlert, rule Rule) error {
	resolutionMsg := fmt.Sprintf(
		"RECOVERED: %s on service '%s' (endpoint: %s) has returned to nominal levels (now %.2f < %.2f)",
		rule.Name,
		alert.Service,
		alert.Endpoint,
		alert.ObservedValue,
		alert.Threshold,
	)

	for _, ch := range rule.Channels {
		switch strings.ToLower(ch) {
		case "sms":
			if cn.atNotifier != nil && cn.oncallStore != nil {
				recipient := cn.oncallStore.CurrentOnCall()
				if recipient != "" {
					smsText := fmt.Sprintf("Observatory RECOVERY: %s on %s is resolved. Metric normalized.", rule.Name, alert.Service)
					_ = cn.atNotifier.SendSMS(ctx, recipient, smsText)
				}
			}
		case "slack":
			if cn.slackNotifier != nil {
				_ = cn.slackNotifier.SendAlert(ctx, rule, alert.ObservedValue, resolutionMsg, alert.ID)
			}
		}
	}

	return nil
}

