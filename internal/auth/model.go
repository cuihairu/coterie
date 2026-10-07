package auth

import (
	"time"

	"github.com/cuihairu/coterie/internal/user"
)

// Session is an issued bearer session. Only the SHA-256 hash of the
// token is stored; the raw token is returned once at register/login.
type Session struct {
	ID        string    `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	UserID    string    `gorm:"column:user_id;not null" json:"user_id"`
	TokenHash string    `gorm:"column:token_hash;not null" json:"-"`
	ExpiresAt time.Time `gorm:"column:expires_at;not null" json:"expires_at"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName aligns the model with the hand-written migration schema.
func (Session) TableName() string { return "sessions" }

// LoginRequest is the payload for POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse is returned by register and login. Token is a bearer
// credential — show it once, store only its hash server-side.
type AuthResponse struct {
	Token string     `json:"token"`
	User  *user.User `json:"user"`
}
