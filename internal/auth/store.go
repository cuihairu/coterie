package auth

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/user"
)

// Store holds the GORM queries for sessions and credential lookup.
type Store struct {
	db *gorm.DB
}

// NewStore builds a Store.
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// CreateSession inserts sess.
func (s *Store) CreateSession(ctx context.Context, sess *Session) error {
	return s.db.WithContext(ctx).Create(sess).Error
}

// SessionByTokenHash returns the session for tokenHash, or nil when
// absent.
func (s *Store) SessionByTokenHash(ctx context.Context, tokenHash string) (*Session, error) {
	var sess Session
	err := s.db.WithContext(ctx).First(&sess, "token_hash = ?", tokenHash).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

// UserByEmail returns the user with the given email
// (case-insensitive, matching the unique index), or nil when absent.
func (s *Store) UserByEmail(ctx context.Context, email string) (*user.User, error) {
	var u user.User
	err := s.db.WithContext(ctx).First(&u, "lower(email) = lower(?)", email).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// DeleteSessionByTokenHash removes the session, ending it everywhere.
func (s *Store) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	return s.db.WithContext(ctx).
		Where("token_hash = ?", tokenHash).
		Delete(&Session{}).Error
}

// DeleteExpiredSessionsForUser drops the user's expired sessions,
// keeping the table bounded as users return.
func (s *Store) DeleteExpiredSessionsForUser(ctx context.Context, userID string) error {
	return s.db.WithContext(ctx).
		Where("user_id = ? AND expires_at < ?", userID, time.Now().UTC()).
		Delete(&Session{}).Error
}
