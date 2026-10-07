package usage

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/cuihairu/coterie/internal/database"
	"github.com/cuihairu/coterie/internal/seat"
	"github.com/cuihairu/coterie/internal/subscription"
	"github.com/cuihairu/coterie/pkg/api"
)

// Store holds the GORM queries for usage records. Every method runs on
// the store's handle, so the service can bind a transaction-scoped
// store with NewStore(tx) and keep the whole write in one tx.
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

// ActiveMember reports whether the member is a current active member of
// the coterie bound to the subscription (invariant 4).
func (s *Store) ActiveMember(ctx context.Context, subscriptionID, memberID string) (bool, error) {
	var n int64
	err := s.db.WithContext(ctx).
		Table("members m").
		Joins("JOIN coteries c ON c.id = m.coterie_id").
		Where("c.subscription_id = ? AND m.id = ? AND m.status = ? AND m.left_at IS NULL",
			subscriptionID, memberID, "active").
		Limit(1).Count(&n).Error
	return n > 0, err
}

// Seat returns the seat with id, or nil when absent.
func (s *Store) Seat(ctx context.Context, id string) (*seat.Seat, error) {
	var st seat.Seat
	err := s.db.WithContext(ctx).First(&st, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// QuotaSeatsOfMember returns the seats the member currently occupies in
// the subscription whose metadata carries a "quota" key.
func (s *Store) QuotaSeatsOfMember(ctx context.Context, subscriptionID, memberID string) ([]seat.Seat, error) {
	var seats []seat.Seat
	err := s.db.WithContext(ctx).
		Where("subscription_id = ? AND member_id = ? AND status = ? AND metadata -> 'quota' IS NOT NULL",
			subscriptionID, memberID, seat.StatusOccupied).
		Order("label ASC, id ASC").
		Find(&seats).Error
	return seats, err
}

// SeatForUpdate loads the seat with a row lock for the used rewrite.
func (s *Store) SeatForUpdate(ctx context.Context, id string) (*seat.Seat, error) {
	var st seat.Seat
	err := s.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&st, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// Create inserts the usage record.
func (s *Store) Create(ctx context.Context, r *UsageRecord) error {
	return s.db.WithContext(ctx).Create(r).Error
}

// ProviderSlugBySubscription returns the slug of the provider behind
// the subscription's product — the key for provider plugin lookups.
func (s *Store) ProviderSlugBySubscription(ctx context.Context, subscriptionID string) (string, error) {
	var slug string
	err := s.db.WithContext(ctx).
		Table("products p").
		Select("pr.slug").
		Joins("JOIN providers pr ON pr.id = p.provider_id").
		Where("p.id = (SELECT product_id FROM subscriptions WHERE id = ?)", subscriptionID).
		Scan(&slug).Error
	return slug, err
}

// RecordByID returns the usage record, or nil when absent.
func (s *Store) RecordByID(ctx context.Context, id string) (*UsageRecord, error) {
	var r UsageRecord
	err := s.db.WithContext(ctx).First(&r, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ListBySubscription returns the subscription's records, newest first,
// narrowed by the filters.
func (s *Store) ListBySubscription(ctx context.Context, subscriptionID string, f ListFilters, page api.Page) ([]UsageRecord, int64, error) {
	q := s.db.WithContext(ctx).Model(&UsageRecord{}).Where("subscription_id = ?", subscriptionID)
	if f.MemberID != "" {
		q = q.Where("member_id = ?", f.MemberID)
	}
	if f.Unit != "" {
		q = q.Where("unit = ?", f.Unit)
	}
	if f.From != nil {
		q = q.Where("recorded_at >= ?", f.From.Time)
	}
	if f.To != nil {
		q = q.Where("recorded_at < ?", f.To.Time.AddDate(0, 0, 1))
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []UsageRecord
	err := q.Order("recorded_at DESC, id ASC").
		Limit(page.Limit).
		Offset(page.Offset).
		Find(&records).Error
	return records, total, err
}

// SumBySeat returns the ledger sum of all records attributed to the
// seat, rendered in the column's text form ("12.5000").
func (s *Store) SumBySeat(ctx context.Context, seatID string) (string, error) {
	var sum string
	err := s.db.WithContext(ctx).
		Raw(`SELECT COALESCE(SUM(amount), 0)::text FROM usage_records WHERE seat_id = ?`, seatID).
		Scan(&sum).Error
	return sum, err
}

// SetSeatMetadata persists the seat's metadata. Callers must hold the
// row lock (SeatForUpdate) in the same transaction.
func (s *Store) SetSeatMetadata(ctx context.Context, seatID string, metadata database.JSONB) error {
	return s.db.WithContext(ctx).Model(&seat.Seat{}).
		Where("id = ?", seatID).
		Update("metadata", metadata).Error
}
