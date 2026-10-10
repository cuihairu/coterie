package marketplace

import (
	"time"

	"github.com/cuihairu/coterie/internal/reputation"
)

// JoinRequest statuses. pending and awaiting_payment are the live
// states (the latter is the payment gate's hold, design D25); every
// decided state keeps the row for history.
const (
	RequestPending         = "pending"
	RequestAwaitingPayment = "awaiting_payment"
	RequestAccepted        = "accepted"
	RequestDeclined        = "declined"
	RequestCancelled       = "cancelled"
)

// JoinRequest is a user's ask to join a listed coterie (design D10).
// The coterie's owner or an admin decides; acceptance admits the user
// through the same checks as invitation acceptance.
type JoinRequest struct {
	ID        string     `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	CoterieID string     `gorm:"column:coterie_id;not null" json:"coterie_id"`
	UserID    string     `gorm:"column:user_id;not null" json:"user_id"`
	Message   string     `gorm:"column:message;not null" json:"message"`
	Status    string     `gorm:"column:status;not null" json:"status"`
	CreatedAt time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	DecidedAt *time.Time `gorm:"column:decided_at" json:"decided_at,omitempty"`
}

// TableName aligns the model with the hand-written migration schema.
func (JoinRequest) TableName() string { return "join_requests" }

// CreateJoinRequestRequest is the payload for POST
// /coteries/{id}/join-requests; the message is optional context for
// the owner.
type CreateJoinRequestRequest struct {
	Message string `json:"message"`
}

// DirectoryEntry is one publicly listed coterie as shown in the
// marketplace browse view. Share estimate is display-only: the
// subscription price floored over the active member count under the
// equal split.
type DirectoryEntry struct {
	CoterieID     string      `gorm:"column:id" json:"coterie_id"`
	Name          string      `gorm:"column:name" json:"name"`
	Status        string      `gorm:"column:status" json:"status"`
	ProductID     string      `gorm:"column:product_id" json:"product_id"`
	ProductName   string      `gorm:"column:product_name" json:"product_name"`
	ProviderName  string      `gorm:"column:provider_name" json:"provider_name"`
	Price         string      `gorm:"column:price" json:"price"`
	Currency      string      `gorm:"column:currency" json:"currency"`
	MemberCount   int         `gorm:"column:member_count" json:"member_count"`
	SeatsTotal    int         `gorm:"column:seats_total" json:"seats_total"`
	SeatsFree     int         `gorm:"column:seats_free" json:"seats_free"`
	OwnerID       string      `gorm:"column:owner_id" json:"-"`
	OwnerUsername string      `gorm:"column:owner_username" json:"-"`
	Owner         *OwnerBadge `gorm:"-" json:"owner"`
	Full          bool        `gorm:"-" json:"full"`
	ShareEstimate string      `gorm:"-" json:"share_estimate"`
}

// OwnerBadge is the directory's owner reputation display slot (design
// D18): a coarse, derived projection of the owner's settlement ledger
// from the same aggregation as the authenticated reputation report.
// Per-currency amounts stay behind that report; publishing the circle
// publicly is the owner's opt-in to this slot.
type OwnerBadge struct {
	UserID        string                        `json:"user_id"`
	Username      string                        `json:"username"`
	Contributions reputation.ContributionCounts `json:"contributions"`
	PaymentRatio  *string                       `json:"payment_ratio"`
}

// JoinRequestView is a join request plus the requester's display name,
// for the owner's inbox.
type JoinRequestView struct {
	JoinRequest
	Username string `json:"username"`
}

// MyJoinRequestView is a request the caller made, joined with the
// target circle's display name for the marketplace card.
type MyJoinRequestView struct {
	JoinRequest
	CoterieName string `json:"coterie_name"`
}

// AdmissionCharge statuses — same lifecycle words as payments.
const (
	ChargePending   = "pending"
	ChargeSucceeded = "succeeded"
	ChargeFailed    = "failed"
)

// AdmissionCharge is a pre-membership receipt (design D25): the payment
// gate takes the requester's estimated share before they join. It lives
// outside the payments ledger on purpose — contributions require
// membership, the gate deliberately precedes it — and covers the
// current period by estimate; real shares keep coming from billing.
type AdmissionCharge struct {
	ID            string     `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	CoterieID     string     `gorm:"column:coterie_id;not null" json:"coterie_id"`
	UserID        string     `gorm:"column:user_id;not null" json:"user_id"`
	JoinRequestID string     `gorm:"column:join_request_id;not null" json:"join_request_id"`
	Amount        string     `gorm:"column:amount" json:"amount"`
	Currency      string     `gorm:"column:currency" json:"currency"`
	Method        string     `gorm:"column:method;not null" json:"method"`
	Status        string     `gorm:"column:status;not null" json:"status"`
	ExternalRef   *string    `gorm:"column:external_ref" json:"external_ref,omitempty"`
	ClientSecret  string     `gorm:"-" json:"client_secret,omitempty"` // async channels only, never stored
	CreatedAt     time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	ConfirmedAt   *time.Time `gorm:"column:confirmed_at" json:"confirmed_at,omitempty"`
}

// TableName aligns the model with the hand-written migration schema.
func (AdmissionCharge) TableName() string { return "admission_charges" }

// StartAdmissionChargeRequest is the payload for POST
// /join-requests/{id}/payments; the amount is computed by the platform.
type StartAdmissionChargeRequest struct {
	Method      string `json:"method"`
	ExternalRef string `json:"external_ref"`
}
