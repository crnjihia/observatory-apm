// path: api/alerts/slack.go
package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// SlackBlock represents a Block Kit layout block.
type SlackBlock struct {
	Type string      `json:"type"`
	Text interface{} `json:"text,omitempty"`
}

// SlackTextObject represents text formatted in mrkdwn or plain_text.
type SlackTextObject struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// SlackPayload represents the JSON payload posted to a Slack incoming webhook.
type SlackPayload struct {
	Text   string       `json:"text"` // Fallback text
	Blocks []SlackBlock `json:"blocks,omitempty"`
}

// SentSlackMessage records outbound Slack alerts.
type SentSlackMessage struct {
	Payload   SlackPayload `json:"payload"`
	SentAt    time.Time    `json:"sent_at"`
	Status    string       `json:"status"`
}

// SlackNotifier handles real-time alerts dispatched to Slack incident channels.
type SlackNotifier struct {
	webhookURL string
	useMock    bool
	httpClient *http.Client

	mu      sync.Mutex
	history []SentSlackMessage
}

// NewSlackNotifier constructs a Slack notifier from environment variables.
func NewSlackNotifier() *SlackNotifier {
	webhookURL := os.Getenv("SLACK_WEBHOOK_URL")
	useMock := os.Getenv("USE_MOCK_ADAPTERS") == "true" || webhookURL == "" || webhookURL == "https://hooks.slack.com/services/MOCK/TOKEN/CHANNEL"

	return &SlackNotifier{
		webhookURL: webhookURL,
		useMock:    useMock,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		history: make([]SentSlackMessage, 0),
	}
}

// SendAlert constructs a rich Block Kit notification and posts it to the webhook.
func (sn *SlackNotifier) SendAlert(ctx context.Context, rule Rule, actualValue float64, description string, incidentID string) error {
	nowEAT := time.Now().In(time.FixedZone("EAT", 3*3600)).Format("15:04:05 Mon 02 Jan 2006")

	icon := "⚠️"
	if rule.Severity == SeverityCritical {
		icon = "🚨"
	}

	headerText := fmt.Sprintf("%s Observatory APM Alert: %s", icon, rule.Name)
	detailText := fmt.Sprintf(
		"*Service:* `%s`\n*Endpoint:* `%s`\n*Metric:* `%s`\n*Observed Value:* `%.2f` (Threshold: `%.2f`)\n*Details:* %s",
		rule.Service,
		rule.Endpoint,
		rule.Metric,
		actualValue,
		rule.Threshold,
		description,
	)

	payload := SlackPayload{
		Text: headerText + " - " + description,
		Blocks: []SlackBlock{
			{
				Type: "header",
				Text: SlackTextObject{
					Type: "plain_text",
					Text: headerText,
				},
			},
			{
				Type: "section",
				Text: SlackTextObject{
					Type: "mrkdwn",
					Text: detailText,
				},
			},
			{
				Type: "context",
				Text: SlackTextObject{
					Type: "mrkdwn",
					Text: fmt.Sprintf("Time: *%s EAT* | Incident ID: `%s`", nowEAT, incidentID),
				},
			},
		},
	}

	if sn.useMock {
		log.Printf("[Slack:MOCK] Webhook payload dispatched for incident %s:\n%s\n", incidentID, payload.Text)
		sn.mu.Lock()
		sn.history = append(sn.history, SentSlackMessage{
			Payload: payload,
			SentAt:  time.Now().UTC(),
			Status:  "Success (Mock)",
		})
		sn.mu.Unlock()
		return nil
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", sn.webhookURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := sn.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("slack webhook network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack webhook returned status %d: %s", resp.StatusCode, string(b))
	}

	sn.mu.Lock()
	sn.history = append(sn.history, SentSlackMessage{
		Payload: payload,
		SentAt:  time.Now().UTC(),
		Status:  "Success",
	})
	sn.mu.Unlock()

	return nil
}

// History returns a copy of outbound Slack payloads.
func (sn *SlackNotifier) History() []SentSlackMessage {
	sn.mu.Lock()
	defer sn.mu.Unlock()
	copied := make([]SentSlackMessage, len(sn.history))
	copy(copied, sn.history)
	return copied
}

