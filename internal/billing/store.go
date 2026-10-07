package billing

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/subscription"
	"github.com/cuihairu/coterie/pkg/api"
)

// Store holds the GORM queries for billing periods and contributions.
type Store struct {
	db *gorm.DB
}

// NewStore builds a Store.
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// DB exposes the underlying handle for transactions in the service.
func (s *Store) DB() *gorm.DB { return s.db }

// SubscriptionByID returns the subscription, or nil when absent.
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

// CreatePeriod inserts the billing period.
func (s *Store) CreatePeriod(ctx context.Context, p *BillingPeriod) error {
	return s.db.WithContext(ctx).Create(p).Error
}

// PeriodByID returns the billing period, or nil when absent.
func (s *Store) PeriodByID(ctx context.Context, id string) (*BillingPeriod, error) {
	var p BillingPeriod
	err := s.db.WithContext(ctx).First(&p, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListPeriods returns the subscription's periods, newest first.
func (s *Store) ListPeriods(ctx context.Context, subscriptionID string, page api.Page) ([]BillingPeriod, int64, error) {
	q := s.db.WithContext(ctx).Model(&BillingPeriod{}).Where("subscription_id = ?", subscriptionID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []BillingPeriod
	err := q.Order("start_date DESC, id DESC").
		Limit(page.Limit).
		Offset(page.Offset).
		Find(&items).Error
	return items, total, err
}

// ClosePeriod moves an open period to closed and reports whether the
// row changed.
func (s *Store) ClosePeriod(ctx context.Context, id string) (bool, error) {
	res := s.db.WithContext(ctx).Model(&BillingPeriod{}).
		Where("id = ? AND status = ?", id, PeriodOpen).
		Update("status", PeriodClosed)
	return res.RowsAffected > 0, res.Error
}

// ActiveMembers returns the active members of the coterie bound to the
// subscription, oldest first — the deterministic order the equal split
// uses to hand out rounding remainders.
func (s *Store) ActiveMembers(ctx context.Context, subscriptionID string) ([]MemberRef, error) {
	var members []MemberRef
	err := s.db.WithContext(ctx).
		Table("members m").
		Joins("JOIN coteries c ON c.id = m.coterie_id").
		Where("c.subscription_id = ? AND m.status = ? AND m.left_at IS NULL", subscriptionID, "active").
		Order("m.joined_at ASC, m.id ASC").
		Scan(&members).Error
	return members, err
}

// MemberRef is the slice of a member row billing needs.
type MemberRef struct {
	ID     string
	UserID string
	Role   string
}

// ActiveMember check for fixed-mode validation.
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

// SeatHold counts occupied seats per member for the subscription, for
// per-seat splitting.
func (s *Store) SeatHolds(ctx context.Context, subscriptionID string) (map[string]int, error) {
	type row struct {
		MemberID string
		N        int
	}
	var rows []row
	err := s.db.WithContext(ctx).Table("seats").
		Select("member_id, COUNT(*) AS n").
		Where("subscription_id = ? AND status = ? AND member_id IS NOT NULL", subscriptionID, "occupied").
		Group("member_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	holds := make(map[string]int, len(rows))
	for _, r := range rows {
		holds[r.MemberID] = r.N
	}
	return holds, nil
}

// CountContributions returns how many contributions the period has.
func (s *Store) CountContributions(ctx context.Context, periodID string) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&Contribution{}).
		Where("billing_period_id = ?", periodID).
		Count(&n).Error
	return n, err
}

// CreateContributions inserts the batch in one statement.
func (s *Store) CreateContributions(ctx context.Context, items []Contribution) error {
	return s.db.WithContext(ctx).Create(&items).Error
}

// ContributionByID returns the contribution, or nil when absent.
func (s *Store) ContributionByID(ctx context.Context, id string) (*Contribution, error) {
	var c Contribution
	err := s.db.WithContext(ctx).First(&c, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListContributions returns the period's contributions, oldest first.
func (s *Store) ListContributions(ctx context.Context, periodID string, page api.Page) ([]Contribution, int64, error) {
	q := s.db.WithContext(ctx).Model(&Contribution{}).Where("billing_period_id = ?", periodID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Contribution
	err := q.Order("created_at ASC, id ASC").
		Limit(page.Limit).
		Offset(page.Offset).
		Find(&items).Error
	return items, total, err
}

// UpdateContribution persists all fields of the contribution.
func (s *Store) UpdateContribution(ctx context.Context, c *Contribution) error {
	return s.db.WithContext(ctx).Save(c).Error
}
