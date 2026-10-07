package notification

import (
	"time"
)

// Notification types, matching the migration CHECK (FR-12).
const (
	TypeInvitation          = "invitation"
	TypePaymentDue          = "payment_due"
	TypeSubscriptionRenewal = "subscription_renewal"
	TypeSeatAssigned        = "seat_assigned"
	TypeSubscriptionExpired = "subscription_expired"
	TypeCoterieClosed       = "coterie_closed"
	TypeSystem              = "system"
	TypeJoinRequested       = "join_requested"
	TypeJoinDecided         = "join_decided"
)

// Notification is one in-app message for a user. Read state is a
// timestamp so the moment of reading survives for free.
type Notification struct {
	ID         string     `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	UserID     string     `gorm:"column:user_id;not null" json:"user_id"`
	Type       string     `gorm:"column:type;not null" json:"type"`
	Title      string     `gorm:"column:title;not null" json:"title"`
	Body       string     `gorm:"column:body;not null" json:"body"`
	EntityType string     `gorm:"column:entity_type" json:"entity_type,omitempty"`
	EntityID   string     `gorm:"column:entity_id" json:"entity_id,omitempty"`
	ReadAt     *time.Time `gorm:"column:read_at" json:"read_at,omitempty"`
	CreatedAt  time.Time  `gorm:"column:created_at" json:"created_at"`
}

// TableName aligns the model with the hand-written migration schema.
func (Notification) TableName() string { return "notifications" }
