package audit

import (
	"context"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/pkg/api"
)

// Store holds the audit read queries. Writes go through Record.
type Store struct {
	db *gorm.DB
}

// NewStore builds a Store.
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// ListBySubscription returns the subscription's audit trail, newest
// first, optionally filtered by action.
func (s *Store) ListBySubscription(ctx context.Context, subscriptionID, action string, page api.Page) ([]Entry, int64, error) {
	return s.list(ctx, "subscription_id", subscriptionID, action, page)
}

// ListByCoterie returns the coterie's audit trail, newest first,
// optionally filtered by action.
func (s *Store) ListByCoterie(ctx context.Context, coterieID, action string, page api.Page) ([]Entry, int64, error) {
	return s.list(ctx, "coterie_id", coterieID, action, page)
}

// ListAll is the platform-wide read (design D27): the whole ledger,
// newest first, filtered by any combination of action, actor, and the
// subscription/coterie pivots.
func (s *Store) ListAll(ctx context.Context, action, actorID, coterieID, subscriptionID string, page api.Page) ([]Entry, int64, error) {
	q := s.db.WithContext(ctx).Model(&Entry{})
	if action != "" {
		q = q.Where("action = ?", action)
	}
	if actorID != "" {
		q = q.Where("actor_user_id = ?", actorID)
	}
	if coterieID != "" {
		q = q.Where("coterie_id = ?", coterieID)
	}
	if subscriptionID != "" {
		q = q.Where("subscription_id = ?", subscriptionID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var entries []Entry
	err := q.Order("created_at DESC, id DESC").
		Limit(page.Limit).
		Offset(page.Offset).
		Find(&entries).Error
	return entries, total, err
}

func (s *Store) list(ctx context.Context, pivot, id, action string, page api.Page) ([]Entry, int64, error) {
	q := s.db.WithContext(ctx).Model(&Entry{}).Where(pivot+" = ?", id)
	if action != "" {
		q = q.Where("action = ?", action)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var entries []Entry
	err := q.Order("created_at DESC, id DESC").
		Limit(page.Limit).
		Offset(page.Offset).
		Find(&entries).Error
	return entries, total, err
}

// SubscriptionOwner returns the subscription's owner, or "" when the
// subscription does not exist. Raw-table read keeps the audit package
// free of core-module imports.
func (s *Store) SubscriptionOwner(ctx context.Context, subscriptionID string) (string, error) {
	var owner string
	err := s.db.WithContext(ctx).
		Table("subscriptions").
		Select("owner_user_id").
		Where("id = ?", subscriptionID).
		Scan(&owner).Error
	return owner, err
}

// CoterieOwner returns the coterie's subscription owner, or "" when
// the coterie does not exist.
func (s *Store) CoterieOwner(ctx context.Context, coterieID string) (string, error) {
	var owner string
	err := s.db.WithContext(ctx).
		Table("coteries c").
		Select("s.owner_user_id").
		Joins("JOIN subscriptions s ON s.id = c.subscription_id").
		Where("c.id = ?", coterieID).
		Scan(&owner).Error
	return owner, err
}
