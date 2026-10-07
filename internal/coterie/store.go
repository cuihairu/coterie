package coterie

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/subscription"
	"github.com/cuihairu/coterie/pkg/api"
)

// Store holds the GORM queries for the coterie aggregate.
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

// CoterieBySubscription returns the coterie bound to the subscription
// (at most one — decision D1), or nil when none exists.
func (s *Store) CoterieBySubscription(ctx context.Context, subscriptionID string) (*Coterie, error) {
	var c Coterie
	err := s.db.WithContext(ctx).First(&c, "subscription_id = ?", subscriptionID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CreateCoterie inserts the coterie.
func (s *Store) CreateCoterie(ctx context.Context, c *Coterie) error {
	return s.db.WithContext(ctx).Create(c).Error
}

// GetCoterie returns the coterie with id, or nil when absent.
func (s *Store) GetCoterie(ctx context.Context, id string) (*Coterie, error) {
	var c Coterie
	err := s.db.WithContext(ctx).First(&c, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListCoteries returns coteries newest first, optionally filtered by
// subscription.
func (s *Store) ListCoteries(ctx context.Context, subscriptionID string, page api.Page) ([]Coterie, int64, error) {
	q := s.db.WithContext(ctx).Model(&Coterie{})
	if subscriptionID != "" {
		q = q.Where("subscription_id = ?", subscriptionID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Coterie
	err := q.Order("created_at DESC, id DESC").
		Limit(page.Limit).
		Offset(page.Offset).
		Find(&items).Error
	return items, total, err
}

// UpdateCoterie persists all fields of the coterie.
func (s *Store) UpdateCoterie(ctx context.Context, c *Coterie) error {
	return s.db.WithContext(ctx).Save(c).Error
}

// CreateMember inserts the member.
func (s *Store) CreateMember(ctx context.Context, m *Member) error {
	return s.db.WithContext(ctx).Create(m).Error
}

// GetMember returns the member with id, or nil when absent.
func (s *Store) GetMember(ctx context.Context, id string) (*Member, error) {
	var m Member
	err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ActiveMemberByUser returns the user's current membership in the
// coterie, or nil when the user is not an active member.
func (s *Store) ActiveMemberByUser(ctx context.Context, coterieID, userID string) (*Member, error) {
	var m Member
	err := s.db.WithContext(ctx).
		First(&m, "coterie_id = ? AND user_id = ? AND left_at IS NULL", coterieID, userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ListMembers returns the coterie's active members, oldest first.
func (s *Store) ListMembers(ctx context.Context, coterieID string, page api.Page) ([]Member, int64, error) {
	q := s.db.WithContext(ctx).Model(&Member{}).
		Where("coterie_id = ? AND left_at IS NULL", coterieID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Member
	err := q.Order("joined_at ASC, id ASC").
		Limit(page.Limit).
		Offset(page.Offset).
		Find(&items).Error
	return items, total, err
}

// MemberCount returns how many active members the coterie has.
func (s *Store) MemberCount(ctx context.Context, coterieID string) (int, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&Member{}).
		Where("coterie_id = ? AND left_at IS NULL", coterieID).
		Count(&n).Error
	return int(n), err
}

// SeatStats returns the subscription's total and free seat counts.
func (s *Store) SeatStats(ctx context.Context, subscriptionID string) (total, free int, err error) {
	type row struct {
		Status string
		N      int
	}
	var rows []row
	err = s.db.WithContext(ctx).Table("seats").
		Select("status, COUNT(*) AS n").
		Where("subscription_id = ?", subscriptionID).
		Group("status").
		Scan(&rows).Error
	for _, r := range rows {
		total += r.N
		if r.Status == "free" {
			free = r.N
		}
	}
	return total, free, err
}

// CreateInvitation inserts the invitation (token already hashed).
func (s *Store) CreateInvitation(ctx context.Context, inv *Invitation) error {
	return s.db.WithContext(ctx).Create(inv).Error
}

// InvitationByToken returns the invitation with the given token hash,
// or nil when unknown.
func (s *Store) InvitationByToken(ctx context.Context, tokenHash string) (*Invitation, error) {
	var inv Invitation
	err := s.db.WithContext(ctx).First(&inv, "token = ?", tokenHash).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

// ListInvitations returns the coterie's invitations, newest first.
func (s *Store) ListInvitations(ctx context.Context, coterieID string, page api.Page) ([]Invitation, int64, error) {
	q := s.db.WithContext(ctx).Model(&Invitation{}).Where("coterie_id = ?", coterieID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Invitation
	err := q.Order("created_at DESC, id DESC").
		Limit(page.Limit).
		Offset(page.Offset).
		Find(&items).Error
	return items, total, err
}

// MarkInvitationAccepted stamps accepted_at exactly once and reports
// whether this call won the race.
func (s *Store) MarkInvitationAccepted(ctx context.Context, id string, now time.Time) (bool, error) {
	res := s.db.WithContext(ctx).Model(&Invitation{}).
		Where("id = ? AND accepted_at IS NULL", id).
		Updates(map[string]any{"accepted_at": now})
	return res.RowsAffected > 0, res.Error
}

// SoftLeftMember sets left_at on the member.
func SoftLeftMember(tx *gorm.DB, memberID string, now time.Time) error {
	return tx.Model(&Member{}).
		Where("id = ?", memberID).
		Update("left_at", now).Error
}
