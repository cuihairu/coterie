package user

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/pkg/api"
)

// Store holds the GORM queries for users.
type Store struct {
	db *gorm.DB
}

// NewStore builds a Store.
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// Create inserts u.
func (s *Store) Create(ctx context.Context, u *User) error {
	return s.db.WithContext(ctx).Create(u).Error
}

// Get returns the user with id, or nil when absent.
func (s *Store) Get(ctx context.Context, id string) (*User, error) {
	var u User
	err := s.db.WithContext(ctx).First(&u, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// List returns users newest first, with the total matching count.
func (s *Store) List(ctx context.Context, page api.Page) ([]User, int64, error) {
	var total int64
	if err := s.db.WithContext(ctx).Model(&User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var users []User
	err := s.db.WithContext(ctx).
		Order("created_at DESC, id DESC").
		Limit(page.Limit).
		Offset(page.Offset).
		Find(&users).Error
	return users, total, err
}
