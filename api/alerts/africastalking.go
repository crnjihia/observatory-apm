// path: api/alerts/africastalking.go
package alerts

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// NormalizeKenyanPhone standardizes phone numbers into E.164 format (+254...).
// Safaricom (070-072, 079, 074, 075, 076), Airtel (073, 078), Telkom (077), and new prefixes (010, 011).
func NormalizeKenyanPhone(input string) (string, error) {
	// Strip spaces, dashes, parentheses
	re := regexp.MustCompile(`[\s\-\(\)]`)
	clean := re.ReplaceAllString(input, "")

	if strings.HasPrefix(clean, "+254") {
		if len(clean) == 13 {
			return clean, nil
		}
		return "", fmt.Errorf("invalid Kenyan E.164 phone number length: %s", input)
	}

	if strings.HasPrefix(clean, "254") {
		if len(clean) == 12 {
			return "+" + clean, nil
		}
		return "", fmt.Errorf("invalid Kenyan phone number length: %s", input)
	}

	if strings.HasPrefix(clean, "07") || strings.HasPrefix(clean, "01") {
		if len(clean) == 10 {
			return "+254" + clean[1:], nil
		}
		return "", fmt.Errorf("invalid local Kenyan phone number length: %s", input)
	}

	return "", fmt.Errorf("unrecognized Kenyan phone number format: %s", input)
}

// SentSMS represents an outbound SMS record captured for logging or testing.
type SentSMS struct {
	Recipient string    `json:"recipient"`
	Message   string    `json:"message"`
	SentAt    time.Time `json:"sent_at"`
	Status    string    `json:"status"`
}

// AfricasTalkingNotifier handles high-priority paging via Africa's Talking SMS API.
type AfricasTalkingNotifier struct {
	username   string
	apiKey     string
	senderID   string
	useMock    bool
	httpClient *http.Client

	mu      sync.Mutex
	history []SentSMS
}

// NewAfricasTalkingNotifier constructs an SMS notifier from environment variables.
func NewAfricasTalkingNotifier() *AfricasTalkingNotifier {
	username := os.Getenv("AFRICAS_TALKING_USERNAME")
	apiKey := os.Getenv("AFRICAS_TALKING_API_KEY")
	senderID := os.Getenv("AFRICAS_TALKING_SENDER_ID")
	if senderID == "" {
		senderID = "OBSERVATORY"
	}

	useMock := os.Getenv("USE_MOCK_ADAPTERS") == "true" || username == "" || apiKey == "" || username == "sandbox"

	return &AfricasTalkingNotifier{
		username: username,
		apiKey:   apiKey,
		senderID: senderID,
		useMock:  useMock,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		history: make([]SentSMS, 0),
	}
}

// SendSMS dispatches an SMS notification to the normalized phone recipient.
func (at *AfricasTalkingNotifier) SendSMS(ctx context.Context, recipient string, message string) error {
	normalized, err := NormalizeKenyanPhone(recipient)
	if err != nil {
		return fmt.Errorf("africas_talking normalization error: %w", err)
	}

	if at.useMock {
		log.Printf("[AfricasTalking:MOCK] SMS dispatched to %s via SenderID '%s':\n%s\n", normalized, at.senderID, message)

		at.mu.Lock()
		at.history = append(at.history, SentSMS{
			Recipient: normalized,
			Message:   message,
			SentAt:    time.Now().UTC(),
			Status:    "Success (Mock)",
		})
		at.mu.Unlock()
		return nil
	}

	// Live Africa's Talking API dispatch
	apiURL := "https://api.africastalking.com/version1/messaging"
	if at.username == "sandbox" {
		apiURL = "https://api.sandbox.africastalking.com/version1/messaging"
	}

	data := url.Values{}
	data.Set("username", at.username)
	data.Set("to", normalized)
	data.Set("message", message)
	if at.senderID != "" && at.senderID != "OBSERVATORY" {
		data.Set("from", at.senderID)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("apiKey", at.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := at.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("africas_talking network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("africas_talking API returned status %d: %s", resp.StatusCode, string(body))
	}

	at.mu.Lock()
	at.history = append(at.history, SentSMS{
		Recipient: normalized,
		Message:   message,
		SentAt:    time.Now().UTC(),
		Status:    "Success",
	})
	at.mu.Unlock()

	return nil
}

// History returns a copy of recently dispatched SMS messages.
func (at *AfricasTalkingNotifier) History() []SentSMS {
	at.mu.Lock()
	defer at.mu.Unlock()
	copied := make([]SentSMS, len(at.history))
	copy(copied, at.history)
	return copied
}

