package billing

import (
	"time"

	"github.com/cuihairu/coterie/internal/database"
)

// Billing period statuses. Closing a period freezes its composition
// (no new generations, no amount edits); settlement status changes
// remain possible afterwards.
const (
	PeriodOpen   = "open"
	PeriodClosed = "closed"
)

// Contribution settlement statuses (design §4.2): the record tracks who
// owes what, decoupled from any payment channel.
const (
	ContributionPending   = "pending"
	ContributionPaid      = "paid"
	ContributionWaived    = "waived"
	ContributionCancelled = "cancelled"
)

// Split modes for generating contributions. Usage-based splitting
// arrives with Phase 2 usage tracking.
const (
	SplitEqual   = "equal"
	SplitPerSeat = "per_seat"
	SplitFixed   = "fixed"
	SplitUsage   = "usage"
)

// BillingPeriod is one chargeable window of a subscription. Periods
// belong to the Subscription aggregate (design §1.3); contributions
// reference them by id only (invariant 6).
type BillingPeriod struct {
	ID             string        `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	SubscriptionID string        `gorm:"column:subscription_id;not null" json:"subscription_id"`
	StartDate      database.Date `gorm:"column:start_date;not null" json:"start_date"`
	EndDate        database.Date `gorm:"column:end_date;not null" json:"end_date"`
	Status         string        `gorm:"column:status;not null" json:"status"`
	CreatedAt      time.Time     `gorm:"column:created_at" json:"created_at"`
}

// TableName aligns the model with the hand-written migration schema.
func (BillingPeriod) TableName() string { return "billing_periods" }

// Contribution is what one member owes for one billing period. The
// currency always equals the subscription's (invariant 5) and there is
// at most one row per member per period (invariant 6).
type Contribution struct {
	ID              string     `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	BillingPeriodID string     `gorm:"column:billing_period_id;not null" json:"billing_period_id"`
	MemberID        string     `gorm:"column:member_id;not null" json:"member_id"`
	Amount          string     `gorm:"column:amount;not null" json:"amount"`
	Currency        string     `gorm:"column:currency;not null" json:"currency"`
	Status          string     `gorm:"column:status;not null" json:"status"`
	PaidAt          *time.Time `gorm:"column:paid_at" json:"paid_at,omitempty"`
	CreatedAt       time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

// TableName aligns the model with the hand-written migration schema.
func (Contribution) TableName() string { return "contributions" }

// CreatePeriodRequest is the payload for POST
// /subscriptions/{id}/billing-periods; explicit dates cover monthly,
// yearly, and custom cycles alike.
type CreatePeriodRequest struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// GenerateRequest is the payload for POST
// /billing-periods/{id}/contributions/generate. Mode defaults to equal;
// fixed requires one item per contributing member.
type GenerateRequest struct {
	Mode  string       `json:"mode"`
	Items []FixedShare `json:"items"`
}

// FixedShare pins one member's amount in fixed mode.
type FixedShare struct {
	MemberID string `json:"member_id"`
	Amount   string `json:"amount"`
}

// UpdateContributionRequest patches the amount and drives settlement
// status (manual settlement — the owner marks what happened).
type UpdateContributionRequest struct {
	Amount *string `json:"amount"`
	Status *string `json:"status"`
}
