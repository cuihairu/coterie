package automation

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/database"
)

// Store holds the scheduler's read queries. Writes go through the
// billing service so automation never bypasses billing invariants.
type Store struct {
	db *gorm.DB
}

// NewStore builds a Store.
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// DB exposes the handle for transaction-free reads in tests.
func (s *Store) DB() *gorm.DB { return s.db }

// DueSubscription is one subscription whose frontier billing period has
// ended and whose cycle the scheduler can extend.
type DueSubscription struct {
	ID           string
	OwnerUserID  string
	BillingCycle string
	CycleDays    *int
	PeriodID     string
	EndDate      database.Date
}

// Due returns auto-billing subscriptions whose frontier period — the
// one with the latest start date — has already ended, with that period.
func (s *Store) Due(ctx context.Context, today time.Time) ([]DueSubscription, error) {
	var out []DueSubscription
	err := s.db.WithContext(ctx).
		Table("subscriptions s").
		Select("s.id, s.owner_user_id, s.billing_cycle, s.cycle_days, p.id AS period_id, p.end_date").
		Joins("JOIN billing_periods p ON p.subscription_id = s.id "+
			"AND p.start_date = (SELECT MAX(p2.start_date) FROM billing_periods p2 WHERE p2.subscription_id = s.id)").
		Where("s.auto_billing AND p.end_date < ?", today).
		Order("p.end_date ASC").
		Scan(&out).Error
	return out, err
}

// PendingContributions counts unsettled contributions left in a period.
func (s *Store) PendingContributions(ctx context.Context, periodID string) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).
		Table("contributions").
		Where("billing_period_id = ? AND status = 'pending'", periodID).
		Count(&n).Error
	return n, err
}
