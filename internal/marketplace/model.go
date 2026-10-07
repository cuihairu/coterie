package marketplace

import (
	"time"
)

// JoinRequest statuses. pending is the only live state; every decision
// is terminal and keeps the row for history.
const (
	RequestPending   = "pending"
	RequestAccepted  = "accepted"
	RequestDeclined  = "declined"
	RequestCancelled = "cancelled"
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
	CoterieID     string `gorm:"column:id" json:"coterie_id"`
	Name          string `gorm:"column:name" json:"name"`
	Status        string `gorm:"column:status" json:"status"`
	ProductID     string `gorm:"column:product_id" json:"product_id"`
	ProductName   string `gorm:"column:product_name" json:"product_name"`
	ProviderName  string `gorm:"column:provider_name" json:"provider_name"`
	Price         string `gorm:"column:price" json:"price"`
	Currency      string `gorm:"column:currency" json:"currency"`
	MemberCount   int    `gorm:"column:member_count" json:"member_count"`
	SeatsTotal    int    `gorm:"column:seats_total" json:"seats_total"`
	SeatsFree     int    `gorm:"column:seats_free" json:"seats_free"`
	Full          bool   `gorm:"-" json:"full"`
	ShareEstimate string `gorm:"-" json:"share_estimate"`
}

// JoinRequestView is a join request plus the requester's display name,
// for the owner's inbox.
type JoinRequestView struct {
	JoinRequest
	Username string `json:"username"`
}
