package dispute

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/pkg/api"
)

// Store holds the dispute persistence helpers.
type Store struct {
	db *gorm.DB
}

// NewStore builds a Store.
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// DB exposes the handle for transactional composition.
func (s *Store) DB() *gorm.DB { return s.db }

// Create inserts the dispute.
func (s *Store) Create(ctx context.Context, d *Dispute) error {
	return s.db.WithContext(ctx).Create(d).Error
}

// ByID returns the dispute, or nil when absent.
func (s *Store) ByID(ctx context.Context, id string) (*Dispute, error) {
	var d Dispute
	err := s.db.WithContext(ctx).First(&d, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// OpenByContribution returns the contribution's open dispute, if any.
func (s *Store) OpenByContribution(ctx context.Context, contributionID string) (*Dispute, error) {
	var d Dispute
	err := s.db.WithContext(ctx).
		Where("contribution_id = ? AND status = ?", contributionID, StatusOpen).
		First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// ContributionContext carries what raising and deciding need to know
// about the disputed contribution.
type ContributionContext struct {
	ContributionID string
	SubscriptionID string
	OwnerUserID    string
	MemberUserID   string
	Status         string
}

// ContributionContext loads the contribution's subscription, owner, and
// paying member — nil when the contribution does not exist.
func (s *Store) ContributionContext(ctx context.Context, contributionID string) (*ContributionContext, error) {
	var out ContributionContext
	err := s.db.WithContext(ctx).
		Table("contributions co").
		Select("co.id AS contribution_id, bp.subscription_id, co.status, "+
			"s.owner_user_id, m.user_id AS member_user_id").
		Joins("JOIN billing_periods bp ON bp.id = co.billing_period_id").
		Joins("JOIN subscriptions s ON s.id = bp.subscription_id").
		Joins("JOIN members m ON m.id = co.member_id").
		Where("co.id = ?", contributionID).
		Scan(&out).Error
	if err != nil {
		return nil, err
	}
	if out.ContributionID == "" {
		return nil, nil
	}
	return &out, nil
}

// IsCoterieMember reports whether the user is an active member of the
// subscription's coterie.
func (s *Store) IsCoterieMember(ctx context.Context, subscriptionID, userID string) (bool, error) {
	var n int64
	err := s.db.WithContext(ctx).
		Table("members m").
		Joins("JOIN coteries c ON c.id = m.coterie_id").
		Where("c.subscription_id = ? AND m.user_id = ? AND m.status = ? AND m.left_at IS NULL",
			subscriptionID, userID, "active").
		Limit(1).Count(&n).Error
	return n > 0, err
}

// ListBySubscription returns the subscription's disputes, newest first,
// optionally narrowed to one status.
func (s *Store) ListBySubscription(ctx context.Context, subscriptionID, status string, page api.Page) ([]Dispute, int64, error) {
	q := s.db.WithContext(ctx).Model(&Dispute{}).Where("subscription_id = ?", subscriptionID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Dispute
	err := q.Order("created_at DESC, id ASC").
		Limit(page.Limit).Offset(page.Offset).
		Find(&items).Error
	return items, total, err
}

// Decide flips an open dispute to the decision in one statement, so a
// concurrent decide loses cleanly. It reports whether the row changed.
func (s *Store) Decide(ctx context.Context, id, decision, note, decidedBy string, decidedAt time.Time) (bool, error) {
	var noteArg *string
	if note != "" {
		noteArg = &note
	}
	q := s.db.WithContext(ctx).Exec(
		"UPDATE disputes SET status = ?, resolution_note = ?, decided_by = ?, decided_at = ? "+
			"WHERE id = ? AND status = ?",
		decision, noteArg, decidedBy, decidedAt, id, StatusOpen)
	if q.Error != nil {
		return false, q.Error
	}
	return q.RowsAffected > 0, nil
}
