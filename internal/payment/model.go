package payment

import (
	"time"
)

// Payment is the settled state of one contribution via one channel.
// Append-only: a correction is a new payment, never an edit.
type Payment struct {
	ID             string     `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	ContributionID string     `gorm:"column:contribution_id;not null" json:"contribution_id"`
	SubscriptionID string     `gorm:"column:subscription_id;not null" json:"subscription_id"`
	PayerUserID    string     `gorm:"column:payer_user_id;not null" json:"payer_user_id"`
	Amount         string     `gorm:"column:amount;not null" json:"amount"`
	Currency       string     `gorm:"column:currency;not null" json:"currency"`
	Method         string     `gorm:"column:method;not null" json:"method"`
	Status         string     `gorm:"column:status;not null" json:"status"`
	ExternalRef    *string    `gorm:"column:external_ref" json:"external_ref,omitempty"`
	PaidAt         *time.Time `gorm:"column:paid_at" json:"paid_at,omitempty"`
	CreatedAt      time.Time  `gorm:"column:created_at" json:"created_at"`

	// ClientSecret is the async channel's completion token (D24) —
	// transient, only set on the response that starts a charge; the
	// channel keeps it retrievable so the ledger never stores it.
	ClientSecret string `gorm:"-" json:"client_secret,omitempty"`
}

// TableName aligns the model with the hand-written migration schema.
func (Payment) TableName() string { return "payments" }

// Payment statuses. Synchronous channels (manual, sandbox) write
// succeeded directly; async channels (D24) start at pending and are
// flipped by the channel's webhook to succeeded or failed.
const (
	StatusPending   = "pending"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

// RecordPaymentRequest is the payload for POST
// /contributions/{id}/payments. Method defaults to the manual adapter;
// external_ref carries a channel reference or a free-text note.
type RecordPaymentRequest struct {
	Method      string `json:"method"`
	ExternalRef string `json:"external_ref"`
}
