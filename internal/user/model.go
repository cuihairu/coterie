package user

import "time"

// User is the platform identity directory entry. M1 keeps it minimal —
// just enough to own subscriptions; authentication arrives in M2.
type User struct {
	ID        string    `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	Username  string    `gorm:"column:username;not null" json:"username"`
	Email     string    `gorm:"column:email;not null" json:"email"`
	AvatarURL *string   `gorm:"column:avatar_url" json:"avatar_url,omitempty"`
	Locale    string    `gorm:"column:locale;not null" json:"locale"`
	Timezone  string    `gorm:"column:timezone;not null" json:"timezone"`
	Status    string    `gorm:"column:status;not null" json:"status"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName aligns the model with the hand-written migration schema.
func (User) TableName() string { return "users" }

const (
	StatusActive    = "active"
	DefaultLocale   = "en"
	DefaultTimezone = "UTC"
)

// CreateUserRequest is the payload for POST /users.
type CreateUserRequest struct {
	Username  string  `json:"username"`
	Email     string  `json:"email"`
	AvatarURL *string `json:"avatar_url"`
	Locale    string  `json:"locale"`
	Timezone  string  `json:"timezone"`
}
