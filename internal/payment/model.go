package payment

import (
	"time"
)

// Payment is the settled state of one contribution via one channel.
// Append-only: a correction is a new payment, never an edit.
type Payment struct {
	ID             string    `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	ContributionID string    `gorm:"column:contribution_id;not null" json:"contribution_id"`
	SubscriptionID string    `gorm:"column:subscription_id;not null" json:"subscription_id"`
	PayerUserID    string    `gorm:"column:payer_user_id;not null" json:"payer_user_id"`
	Amount         string    `gorm:"column:amount;not null" json:"amount"`
	Currency       string    `gorm:"column:currency;not null" json:"currency"`
	Method         string    `gorm:"column:method;not null" json:"method"`
	Status         string    `gorm:"column:status;not null" json:"status"`
	ExternalRef    *string   `gorm:"column:external_ref" json:"external_ref,omitempty"`
	PaidAt         time.Time `gorm:"column:paid_at;not null" json:"paid_at"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName aligns the model with the hand-written migration schema.
func (Payment) TableName() string { return "payments" }

// Payment statuses. Only succeeded exists today — a failed charge never
// reaches the ledger.
const StatusSucceeded = "succeeded"

// RecordPaymentRequest is the payload for POST
// /contributions/{id}/payments. Method defaults to the manual adapter;
// external_ref carries a channel reference or a free-text note.
type RecordPaymentRequest struct {
	Method      string `json:"method"`
	ExternalRef string `json:"external_ref"`
}
