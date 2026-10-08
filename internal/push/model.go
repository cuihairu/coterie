package push

import "time"

// Subscription is one browser push endpoint a user registered for Web
// Push delivery (design D22). P256dh/Auth hold the client key material
// verbatim (base64url) — the server needs the original strings to
// encrypt payloads under RFC 8291.
type Subscription struct {
	ID        string    `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	UserID    string    `gorm:"column:user_id;not null" json:"user_id"`
	Endpoint  string    `gorm:"column:endpoint;not null" json:"endpoint"`
	P256dh    string    `gorm:"column:p256dh;not null" json:"-"`
	Auth      string    `gorm:"column:auth;not null" json:"-"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

// TableName aligns the model with the hand-written migration schema.
func (Subscription) TableName() string { return "push_subscriptions" }

// RegisterRequest is the payload for POST /push/subscriptions: the
// browser's PushSubscription JSON, unchanged.
type RegisterRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}
