package coterie

import (
	"time"
)

// Coterie lifecycle states (design §1.5). Full is NOT persisted — it is
// derived from the free seat count (decision D3).
const (
	StatusDraft  = "draft"
	StatusOpen   = "open"
	StatusActive = "active"
	StatusPaused = "paused"
	StatusClosed = "closed"
)

// Member roles within a coterie.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// Coterie is the collaboration structure around exactly one subscription
// (decision D1). It stores no capacity (invariant 3) and no credentials.
type Coterie struct {
	ID             string    `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	SubscriptionID string    `gorm:"column:subscription_id;not null" json:"subscription_id"`
	Name           string    `gorm:"column:name;not null" json:"name"`
	Status         string    `gorm:"column:status;not null" json:"status"`
	Listing        string    `gorm:"column:listing;not null" json:"listing"`
	PaymentGate    bool      `gorm:"column:payment_gate;not null" json:"payment_gate"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName aligns the model with the hand-written migration schema.
func (Coterie) TableName() string { return "coteries" }

// Member is a platform user's participation in a coterie. Leaving is a
// soft record (left_at) so history survives for settlement.
type Member struct {
	ID        string     `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	CoterieID string     `gorm:"column:coterie_id;not null" json:"coterie_id"`
	UserID    string     `gorm:"column:user_id;not null" json:"user_id"`
	Role      string     `gorm:"column:role;not null" json:"role"`
	Status    string     `gorm:"column:status;not null" json:"status"`
	JoinedAt  time.Time  `gorm:"column:joined_at" json:"joined_at"`
	LeftAt    *time.Time `gorm:"column:left_at" json:"left_at,omitempty"`
}

// TableName aligns the model with the hand-written migration schema.
func (Member) TableName() string { return "members" }

// Invitation carries the hashed join token. The raw token is shown to
// the creator once and never stored (same scheme as auth sessions).
type Invitation struct {
	ID         string     `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	CoterieID  string     `gorm:"column:coterie_id;not null" json:"coterie_id"`
	Token      string     `gorm:"column:token;not null" json:"-"`
	Role       string     `gorm:"column:role;not null" json:"role"`
	ExpireAt   time.Time  `gorm:"column:expire_at;not null" json:"expire_at"`
	CreatedBy  string     `gorm:"column:created_by;not null" json:"created_by"`
	AcceptedAt *time.Time `gorm:"column:accepted_at" json:"accepted_at,omitempty"`
	CreatedAt  time.Time  `gorm:"column:created_at" json:"created_at"`
}

// TableName aligns the model with the hand-written migration schema.
func (Invitation) TableName() string { return "invitations" }

// CoterieView is a coterie plus its derived display state: the member
// count, the seat summary, and the full flag (decision D3 — derived,
// never stored).
type CoterieView struct {
	Coterie
	MemberCount int  `json:"member_count"`
	SeatsTotal  int  `json:"seats_total"`
	SeatsFree   int  `json:"seats_free"`
	Full        bool `json:"full"`
}

// CreateCoterieRequest is the payload for POST /coteries. Capacity is
// "how many seats to provision for this subscription" (invariant 3) —
// the coterie itself never stores it.
type CreateCoterieRequest struct {
	SubscriptionID string `json:"subscription_id"`
	Name           string `json:"name"`
	Capacity       int    `json:"capacity"`
}

// UpdateCoterieRequest patches the name and drives the lifecycle via
// status (illegal transitions are rejected with 409).
type UpdateCoterieRequest struct {
	Name        *string `json:"name"`
	Status      *string `json:"status"`
	Listing     *string `json:"listing"`      // private or public (design D10)
	PaymentGate *bool   `json:"payment_gate"` // require payment before admission (design D25)
}

// Listing values. Public coteries appear in the marketplace directory
// and accept join requests; private is the default.
const (
	ListingPrivate = "private"
	ListingPublic  = "public"
)

// CreateInvitationRequest is the payload for POST
// /coteries/{id}/invitations. Role accepts admin and member only.
type CreateInvitationRequest struct {
	Role          string `json:"role"`
	ExpiresInDays int    `json:"expires_in_days"`
}

// InvitationView is an invitation plus the raw token — populated only
// in the create response, never again.
type InvitationView struct {
	Invitation
	Token string `json:"token,omitempty"`
}

// AcceptInvitationRequest is the payload for POST /invitations/accept.
type AcceptInvitationRequest struct {
	Token string `json:"token"`
}

// Block is the owner's refusal of a user on this coterie (design D26).
// The composite primary key makes it pure set membership — one row per
// (coterie, user) pair, nothing else to say.
type Block struct {
	CoterieID string    `gorm:"column:coterie_id;primaryKey;type:uuid" json:"coterie_id"`
	UserID    string    `gorm:"column:user_id;primaryKey;type:uuid" json:"user_id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName aligns the model with the hand-written migration schema.
func (Block) TableName() string { return "coterie_blocks" }

// BlockEntry is a block with the blocked user's display identity — the
// shape of the owner's block list.
type BlockEntry struct {
	CoterieID string    `gorm:"column:coterie_id" json:"coterie_id"`
	UserID    string    `gorm:"column:user_id" json:"user_id"`
	Username  string    `gorm:"column:username" json:"username"`
	Email     string    `gorm:"column:email" json:"email"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}
