package seat

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/subscription"
	"github.com/cuihairu/coterie/pkg/api"
)

// Store holds the GORM queries for seats.
type Store struct {
	db *gorm.DB
}

// NewStore builds a Store.
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// SubscriptionByID returns the owning subscription, or nil when absent.
func (s *Store) SubscriptionByID(ctx context.Context, id string) (*subscription.Subscription, error) {
	var sub subscription.Subscription
	err := s.db.WithContext(ctx).First(&sub, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// CountBySubscription returns how many seats the subscription has.
func (s *Store) CountBySubscription(ctx context.Context, subscriptionID string) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&Seat{}).
		Where("subscription_id = ?", subscriptionID).
		Count(&n).Error
	return n, err
}

// Create inserts seats in one batch.
func (s *Store) Create(ctx context.Context, seats []Seat) error {
	return s.db.WithContext(ctx).Create(&seats).Error
}

// Get returns the seat with id, or nil when absent.
func (s *Store) Get(ctx context.Context, id string) (*Seat, error) {
	var seat Seat
	err := s.db.WithContext(ctx).First(&seat, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &seat, nil
}

// ListBySubscription returns the subscription's seats ordered by label.
func (s *Store) ListBySubscription(ctx context.Context, subscriptionID string, page api.Page) ([]Seat, int64, error) {
	q := s.db.WithContext(ctx).Model(&Seat{}).Where("subscription_id = ?", subscriptionID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var seats []Seat
	err := q.Order("label ASC, id ASC").
		Limit(page.Limit).
		Offset(page.Offset).
		Find(&seats).Error
	return seats, total, err
}

// Update persists all fields of the seat.
func (s *Store) Update(ctx context.Context, seat *Seat) error {
	return s.db.WithContext(ctx).Save(seat).Error
}

// Occupy moves a free seat to occupied with the given assignee and
// reports whether the row changed.
func (s *Store) Occupy(ctx context.Context, id, memberID string) (bool, error) {
	res := s.db.WithContext(ctx).Model(&Seat{}).
		Where("id = ? AND status = ?", id, StatusFree).
		Updates(map[string]any{"member_id": memberID, "status": StatusOccupied})
	return res.RowsAffected > 0, res.Error
}

// Release moves an occupied seat back to free and reports whether the
// row changed.
func (s *Store) Release(ctx context.Context, id string) (bool, error) {
	res := s.db.WithContext(ctx).Model(&Seat{}).
		Where("id = ? AND status = ?", id, StatusOccupied).
		Updates(map[string]any{"member_id": nil, "status": StatusFree})
	return res.RowsAffected > 0, res.Error
}

// ReleaseByMember frees every seat the member occupies and returns how
// many rows changed. Used when a member leaves or is removed.
func (s *Store) ReleaseByMember(ctx context.Context, memberID string) (int64, error) {
	res := s.db.WithContext(ctx).Model(&Seat{}).
		Where("member_id = ?", memberID).
		Updates(map[string]any{"member_id": nil, "status": StatusFree})
	return res.RowsAffected, res.Error
}

// ActiveMemberInCoterie reports whether the member is a current active
// member of the coterie bound to the subscription (invariant 4).
func (s *Store) ActiveMemberInCoterie(ctx context.Context, subscriptionID, memberID string) (bool, error) {
	var one int64
	err := s.db.WithContext(ctx).Table("members m").
		Joins("JOIN coteries c ON c.id = m.coterie_id").
		Where("c.subscription_id = ? AND m.id = ? AND m.status = ? AND m.left_at IS NULL",
			subscriptionID, memberID, "active").
		Limit(1).
		Count(&one).Error
	return one > 0, err
}
