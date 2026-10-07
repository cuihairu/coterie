// Package dispute implements FR-17: a member challenges a
// contribution, the subscription owner decides. The dispute ledger is
// decoupled from money movement — deciding never touches the
// contribution; the owner adjusts settlement through the existing
// billing surface if the dispute calls for it.
package dispute

import (
	"time"
)

// Dispute statuses: open until the owner decides; deciding is one-way.
const (
	StatusOpen     = "open"
	StatusResolved = "resolved"
	StatusRejected = "rejected"
)

// Decisions accepted by the decide endpoint.
const (
	DecideResolved = "resolved"
	DecideRejected = "rejected"
)

// Dispute is one challenge against a contribution.
type Dispute struct {
	ID             string     `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	ContributionID string     `gorm:"column:contribution_id;not null" json:"contribution_id"`
	SubscriptionID string     `gorm:"column:subscription_id;not null" json:"subscription_id"`
	RaisedBy       string     `gorm:"column:raised_by;not null" json:"raised_by"`
	Reason         string     `gorm:"column:reason;not null" json:"reason"`
	Evidence       *string    `gorm:"column:evidence" json:"evidence,omitempty"`
	Status         string     `gorm:"column:status;not null" json:"status"`
	ResolutionNote *string    `gorm:"column:resolution_note" json:"resolution_note,omitempty"`
	DecidedBy      *string    `gorm:"column:decided_by" json:"decided_by,omitempty"`
	CreatedAt      time.Time  `gorm:"column:created_at" json:"created_at"`
	DecidedAt      *time.Time `gorm:"column:decided_at" json:"decided_at,omitempty"`
}

// TableName aligns the model with the hand-written migration schema.
func (Dispute) TableName() string { return "disputes" }

// RaiseRequest is the payload for POST /contributions/{id}/disputes.
type RaiseRequest struct {
	Reason   string `json:"reason"`
	Evidence string `json:"evidence"`
}

// DecideRequest is the payload for POST /disputes/{id}/decide.
type DecideRequest struct {
	Decision string `json:"decision"`
	Note     string `json:"note"`
}
