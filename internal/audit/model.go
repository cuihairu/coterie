// Package audit implements FR-15 (design D17): an append-only ledger
// of who did what, when, to which entity, with before/after snapshots.
// Money- and membership-relevant actions must always be audited, so
// entries are recorded inside the same business transaction as the
// mutation they describe — an audit failure rolls the mutation back.
// The read surface is owner-only; there is no edit surface.
package audit

import (
	"time"

	"github.com/cuihairu/coterie/internal/database"
)

// Actions recorded by the core. The migration's CHECK constraint is
// the authority; this set mirrors it for filter validation.
const (
	ActionCoterieCreated      = "coterie_created"
	ActionCoterieUpdated      = "coterie_updated"
	ActionMemberRemoved       = "member_removed"
	ActionMemberLeft          = "member_left"
	ActionSeatAssigned        = "seat_assigned"
	ActionSeatReleased        = "seat_released"
	ActionSeatUpdated         = "seat_updated"
	ActionSubscriptionUpdated = "subscription_updated"
	ActionContributionUpdated = "contribution_updated"
	ActionPaymentRecorded     = "payment_recorded"
	ActionPeriodClosed        = "period_closed"
)

// Actions is the known action set, keyed for filter checks.
var Actions = map[string]bool{
	ActionCoterieCreated:      true,
	ActionCoterieUpdated:      true,
	ActionMemberRemoved:       true,
	ActionMemberLeft:          true,
	ActionSeatAssigned:        true,
	ActionSeatReleased:        true,
	ActionSeatUpdated:         true,
	ActionSubscriptionUpdated: true,
	ActionContributionUpdated: true,
	ActionPaymentRecorded:     true,
	ActionPeriodClosed:        true,
}

// Entry is one immutable audit record. The table's shape comes from
// the M1 skeleton (0001_init): identity id, actor_user_id, and
// before_state/after_state; migration 0011 adds the query pivots and
// the action CHECK.
type Entry struct {
	ID             int64           `gorm:"column:id;primaryKey" json:"id"`
	ActorID        *string         `gorm:"column:actor_user_id" json:"actor_id,omitempty"`
	Action         string          `gorm:"column:action;not null" json:"action"`
	EntityType     string          `gorm:"column:entity_type;not null" json:"entity_type"`
	EntityID       string          `gorm:"column:entity_id" json:"entity_id"`
	SubscriptionID *string         `gorm:"column:subscription_id" json:"subscription_id,omitempty"`
	CoterieID      *string         `gorm:"column:coterie_id" json:"coterie_id,omitempty"`
	Before         *database.JSONB `gorm:"column:before_state" json:"before,omitempty"`
	After          *database.JSONB `gorm:"column:after_state" json:"after,omitempty"`
	CreatedAt      time.Time       `gorm:"column:created_at" json:"created_at"`
}

// TableName aligns the model with the hand-written migration schema.
func (Entry) TableName() string { return "audit_logs" }
