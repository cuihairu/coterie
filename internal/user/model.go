package user

import "time"

// User is the platform identity directory entry. PasswordHash stays
// nil for users registered through future OAuth/Passkey flows.
type User struct {
	ID           string    `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	Username     string    `gorm:"column:username;not null" json:"username"`
	Email        string    `gorm:"column:email;not null" json:"email"`
	PasswordHash *string   `gorm:"column:password_hash" json:"-"`
	AvatarURL    *string   `gorm:"column:avatar_url" json:"avatar_url,omitempty"`
	Locale       string    `gorm:"column:locale;not null" json:"locale"`
	Timezone     string    `gorm:"column:timezone;not null" json:"timezone"`
	Status       string    `gorm:"column:status;not null" json:"status"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName aligns the model with the hand-written migration schema.
func (User) TableName() string { return "users" }

const (
	StatusActive    = "active"
	DefaultLocale   = "en"
	DefaultTimezone = "UTC"
)

// CreateUserRequest is the payload for POST /users and
// POST /auth/register. Password is optional: when set (registration) a
// bcrypt hash is stored, otherwise the user has no local credentials.
type CreateUserRequest struct {
	Username  string  `json:"username"`
	Email     string  `json:"email"`
	Password  string  `json:"password"`
	AvatarURL *string `json:"avatar_url"`
	Locale    string  `json:"locale"`
	Timezone  string  `json:"timezone"`
}
