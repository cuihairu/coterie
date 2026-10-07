package subscription

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/product"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// Store holds the GORM queries for subscriptions.
type Store struct {
	db *gorm.DB
}

// NewStore builds a Store.
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// Create inserts s.
func (s *Store) Create(ctx context.Context, sub *Subscription) error {
	return s.db.WithContext(ctx).Create(sub).Error
}

// Get returns the subscription with id, or nil when absent.
func (s *Store) Get(ctx context.Context, id string) (*Subscription, error) {
	var sub Subscription
	err := s.db.WithContext(ctx).First(&sub, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// ProductExists reports whether the referenced product exists.
func (s *Store) ProductExists(ctx context.Context, id string) (bool, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&product.Product{}).
		Where("id = ?", id).
		Count(&n).Error
	return n > 0, err
}

// UserExists reports whether the referenced owner exists.
func (s *Store) UserExists(ctx context.Context, id string) (bool, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&user.User{}).
		Where("id = ?", id).
		Count(&n).Error
	return n > 0, err
}

// List returns subscriptions newest first, optionally filtered by owner
// and product.
func (s *Store) List(ctx context.Context, ownerUserID, productID string, page api.Page) ([]Subscription, int64, error) {
	q := s.db.WithContext(ctx).Model(&Subscription{})
	if ownerUserID != "" {
		q = q.Where("owner_user_id = ?", ownerUserID)
	}
	if productID != "" {
		q = q.Where("product_id = ?", productID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var subs []Subscription
	err := q.Order("created_at DESC, id DESC").
		Limit(page.Limit).
		Offset(page.Offset).
		Find(&subs).Error
	return subs, total, err
}

// Update persists all fields of sub.
func (s *Store) Update(ctx context.Context, sub *Subscription) error {
	return s.db.WithContext(ctx).Save(sub).Error
}

// Delete removes the subscription and reports whether a row was
// removed.
func (s *Store) Delete(ctx context.Context, id string) (int64, error) {
	res := s.db.WithContext(ctx).Delete(&Subscription{}, "id = ?", id)
	return res.RowsAffected, res.Error
}
