package seat

import (
	"encoding/json"
	"time"

	"github.com/cuihairu/coterie/internal/database"
)

// Seat statuses. A seat is occupied if and only if it has an assignee —
// the database CHECK enforces the pairing (design §1.6).
const (
	StatusFree     = "free"
	StatusOccupied = "occupied"
	StatusDisabled = "disabled"
)

// Seat is one unit of the subscription's shareable capacity. Seats belong
// to the Subscription (decision D2); member_id points at an active member
// of the coterie bound to the same subscription (invariant 4). For quota
// sharing the seat carries its allowance in metadata (decision D4).
type Seat struct {
	ID             string         `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	SubscriptionID string         `gorm:"column:subscription_id;not null" json:"subscription_id"`
	MemberID       *string        `gorm:"column:member_id" json:"member_id,omitempty"`
	Label          string         `gorm:"column:label;not null" json:"label"`
	Status         string         `gorm:"column:status;not null" json:"status"`
	Metadata       database.JSONB `gorm:"column:metadata;not null" json:"metadata"`
	CreatedAt      time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"column:updated_at" json:"updated_at"`
}

// TableName aligns the model with the hand-written migration schema.
func (Seat) TableName() string { return "seats" }

// ProvisionRequest is the payload for creating seats under a
// subscription; count seats are appended up to subscription.max_seats.
type ProvisionRequest struct {
	Count int `json:"count"`
}

// AssignRequest is the payload for POST /seats/{id}/assign.
type AssignRequest struct {
	MemberID string `json:"member_id"`
}

// UpdateSeatRequest patches mutable seat fields. Status accepts only
// free and disabled — occupying a seat goes through /assign so the
// member linkage and the status stay consistent.
type UpdateSeatRequest struct {
	Label    *string         `json:"label"`
	Metadata json.RawMessage `json:"metadata"`
	Status   *string         `json:"status"`
}
