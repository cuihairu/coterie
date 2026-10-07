package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// webhookTimeout bounds one POST.
const webhookTimeout = 10 * time.Second

// WebhookChannel POSTs every notification as JSON to a fixed URL —
// the platform-level outbound hook (design §10). The body is
// HMAC-SHA256-signed when a secret is configured so receivers can
// verify authenticity.
type WebhookChannel struct {
	URL    string
	Secret string
	client *http.Client
}

// NewWebhookChannel builds the channel; URL empty means disabled.
func NewWebhookChannel(url, secret string) *WebhookChannel {
	return &WebhookChannel{
		URL:    url,
		Secret: secret,
		client: &http.Client{Timeout: webhookTimeout},
	}
}

// Name identifies the channel in delivery logs.
func (c *WebhookChannel) Name() string { return "webhook" }

// Enabled reports whether configuration is present.
func (c *WebhookChannel) Enabled() bool { return c.URL != "" }

// webhookPayload is the JSON body posted for one notification.
type webhookPayload struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	UserID     string `json:"user_id"`
	EntityType string `json:"entity_type,omitempty"`
	EntityID   string `json:"entity_id,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// Deliver posts the payload; any 2xx counts as delivered.
func (c *WebhookChannel) Deliver(ctx context.Context, n *Notification) error {
	payload, err := json.Marshal(webhookPayload{
		ID:         n.ID,
		Type:       n.Type,
		Title:      n.Title,
		Body:       n.Body,
		UserID:     n.UserID,
		EntityType: n.EntityType,
		EntityID:   n.EntityID,
		CreatedAt:  n.CreatedAt.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Coterie-Event", n.Type)
	if c.Secret != "" {
		req.Header.Set("X-Coterie-Signature", sign(payload, c.Secret))
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("webhook receiver answered %d", resp.StatusCode)
	}
	return nil
}

// sign returns the hex HMAC-SHA256 of the body.
func sign(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
