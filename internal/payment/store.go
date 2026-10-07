package payment

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/billing"
	"github.com/cuihairu/coterie/pkg/api"
)

// Store holds the GORM queries for the payments ledger.
type Store struct {
	db *gorm.DB
}

// NewStore builds a Store.
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// DB exposes the raw handle for transaction scoping.
func (s *Store) DB() *gorm.DB { return s.db }

// contributionSettled carries the contribution plus the ids the
// permission and routing decisions need: who owns the subscription and
// who the paying member is.
type contributionSettled struct {
	ID              string
	BillingPeriodID string
	MemberID        string
	Amount          string
	Currency        string
	Status          string
	SubscriptionID  string
	OwnerUserID     string
	MemberUserID    string
}

// ContributionForPayment resolves a contribution with its subscription
// owner and payer user, or nil when absent.
func (s *Store) ContributionForPayment(ctx context.Context, id string) (*contributionSettled, error) {
	var c contributionSettled
	err := s.db.WithContext(ctx).
		Table("contributions c").
		Select(`c.id, c.billing_period_id, c.member_id, c.amount, c.currency, c.status,
			bp.subscription_id, s.owner_user_id, m.user_id AS member_user_id`).
		Joins("JOIN billing_periods bp ON bp.id = c.billing_period_id").
		Joins("JOIN subscriptions s ON s.id = bp.subscription_id").
		Joins("JOIN members m ON m.id = c.member_id").
		Where("c.id = ?", id).
		Scan(&c).Error
	if err != nil {
		return nil, err
	}
	if c.ID == "" {
		return nil, nil
	}
	return &c, nil
}

// Create inserts the payment row.
func (s *Store) Create(ctx context.Context, p *Payment) error {
	return s.db.WithContext(ctx).Create(p).Error
}

// PaymentByID returns the payment, or nil when absent.
func (s *Store) PaymentByID(ctx context.Context, id string) (*Payment, error) {
	var p Payment
	err := s.db.WithContext(ctx).First(&p, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListByContribution returns the contribution's payments, newest first.
func (s *Store) ListByContribution(ctx context.Context, contributionID string, page api.Page) ([]Payment, int64, error) {
	q := s.db.WithContext(ctx).Model(&Payment{}).Where("contribution_id = ?", contributionID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Payment
	err := q.Order("paid_at DESC, id ASC").
		Limit(page.Limit).
		Offset(page.Offset).
		Scan(&items).Error
	return items, total, err
}

// SetContributionPaid flips a pending contribution to paid — the
// receivable side of the adapter contract. Reports whether the row
// changed.
func (s *Store) SetContributionPaid(ctx context.Context, contributionID string, paidAt time.Time) (bool, error) {
	res := s.db.WithContext(ctx).
		Exec(`UPDATE contributions SET status = ?, paid_at = ?, updated_at = ?
			WHERE id = ? AND status = ?`,
			billing.ContributionPaid, paidAt, paidAt, contributionID, billing.ContributionPending)
	return res.RowsAffected > 0, res.Error
}
