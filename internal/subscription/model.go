package subscription

import (
	"encoding/json"
	"time"

	"github.com/cuihairu/coterie/internal/database"
)

// Subscription statuses persisted in M1. The subscription lifecycle is
// independent of the coterie lifecycle (design §1.7).
const (
	StatusActive    = "active"
	StatusPaused    = "paused"
	StatusCancelled = "cancelled"
	StatusExpired   = "expired"
)

// Billing cycles accepted for billing_cycle.
var BillingCycles = map[string]bool{
	"monthly": true,
	"yearly":  true,
	"custom":  true,
}

// SharingModes determines Seat semantics (design §1.6 / §3.3).
var SharingModes = map[string]bool{
	"account":  true,
	"seat":     true,
	"family":   true,
	"quota":    true,
	"resource": true,
}

// Subscription is the aggregate root and the single source of cost and
// capacity: it owns seats, the sharing policy, and billing periods.
// Money is carried as a decimal string ("12.99"); dates as "YYYY-MM-DD".
type Subscription struct {
	ID            string         `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	ProductID     string         `gorm:"column:product_id;not null" json:"product_id"`
	OwnerUserID   string         `gorm:"column:owner_user_id;not null" json:"owner_user_id"`
	BillingCycle  string         `gorm:"column:billing_cycle;not null" json:"billing_cycle"`
	Price         string         `gorm:"column:price;not null" json:"price"`
	Currency      string         `gorm:"column:currency;not null" json:"currency"`
	StartDate     database.Date  `gorm:"column:start_date;not null" json:"start_date"`
	RenewalDate   *database.Date `gorm:"column:renewal_date" json:"renewal_date,omitempty"`
	Status        string         `gorm:"column:status;not null" json:"status"`
	MaxSeats      int            `gorm:"column:max_seats;not null" json:"max_seats"`
	MaxMembers    *int           `gorm:"column:max_members" json:"max_members,omitempty"`
	SharingPolicy database.JSONB `gorm:"column:sharing_policy;not null" json:"sharing_policy"`
	CreatedAt     time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"column:updated_at" json:"updated_at"`
}

// TableName aligns the model with the hand-written migration schema.
func (Subscription) TableName() string { return "subscriptions" }

// CreateSubscriptionRequest is the payload for POST /subscriptions.
// Dates arrive as "YYYY-MM-DD" strings and price as a decimal string.
type CreateSubscriptionRequest struct {
	ProductID     string          `json:"product_id"`
	OwnerUserID   string          `json:"owner_user_id"`
	BillingCycle  string          `json:"billing_cycle"`
	Price         string          `json:"price"`
	Currency      string          `json:"currency"`
	StartDate     string          `json:"start_date"`
	RenewalDate   *string         `json:"renewal_date"`
	MaxSeats      int             `json:"max_seats"`
	MaxMembers    *int            `json:"max_members"`
	SharingPolicy json.RawMessage `json:"sharing_policy"`
}

// UpdateSubscriptionRequest patches mutable fields; nil fields are left
// unchanged. renewal_date "" clears the field.
type UpdateSubscriptionRequest struct {
	BillingCycle  *string          `json:"billing_cycle"`
	Price         *string          `json:"price"`
	Status        *string          `json:"status"`
	RenewalDate   *string          `json:"renewal_date"`
	MaxSeats      *int             `json:"max_seats"`
	MaxMembers    *int             `json:"max_members"`
	SharingPolicy *json.RawMessage `json:"sharing_policy"`
}
