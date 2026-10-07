package marketplace

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/coterie"
	"github.com/cuihairu/coterie/pkg/api"
)

// Store holds the GORM queries for the marketplace read model and join
// requests.
type Store struct {
	db *gorm.DB
}

// NewStore builds a Store.
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// Directory lists publicly listed, recruiting coteries with their
// product/provider names and price, oldest first.
func (s *Store) Directory(ctx context.Context, productID string, page api.Page) ([]DirectoryEntry, int64, error) {
	q := s.db.WithContext(ctx).
		Table("coteries c").
		Joins("JOIN subscriptions s ON s.id = c.subscription_id").
		Joins("JOIN products p ON p.id = s.product_id").
		Joins("JOIN providers pr ON pr.id = p.provider_id").
		Where("c.listing = ? AND c.status IN ('open', 'active')", "public")
	if productID != "" {
		q = q.Where("s.product_id = ?", productID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var entries []DirectoryEntry
	err := q.
		Select(`c.id, c.name, c.status, s.product_id, p.name AS product_name,
			pr.name AS provider_name, s.price, s.currency,
			(SELECT COUNT(*) FROM members m WHERE m.coterie_id = c.id AND m.left_at IS NULL) AS member_count,
			(SELECT COUNT(*) FROM seats st WHERE st.subscription_id = c.subscription_id) AS seats_total,
			(SELECT COUNT(*) FROM seats st WHERE st.subscription_id = c.subscription_id AND st.status = 'free') AS seats_free`).
		Order("c.created_at ASC, c.id ASC").
		Limit(page.Limit).
		Offset(page.Offset).
		Scan(&entries).Error
	return entries, total, err
}

// CoterieByID returns the raw coterie row, or nil when absent. Plain
// read lookup — visibility and admission decisions stay with the
// coterie module's rules.
func (s *Store) CoterieByID(ctx context.Context, id string) (*coterie.Coterie, error) {
	var c coterie.Coterie
	err := s.db.WithContext(ctx).First(&c, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ActiveMemberRole reports the actor's role when they are a current
// active member of the coterie.
func (s *Store) ActiveMemberRole(ctx context.Context, coterieID, userID string) (string, bool, error) {
	var m coterie.Member
	err := s.db.WithContext(ctx).
		First(&m, "coterie_id = ? AND user_id = ? AND left_at IS NULL", coterieID, userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return m.Role, true, nil
}

// SeatStats returns the subscription's total and free seat counts.
func (s *Store) SeatStats(ctx context.Context, subscriptionID string) (total, free int, err error) {
	type row struct {
		Status string
		N      int
	}
	var rows []row
	err = s.db.WithContext(ctx).
		Table("seats").
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

// CoterieOwnerUserID returns the platform user owning the coterie —
// the subscription owner (decision D5).
func (s *Store) CoterieOwnerUserID(ctx context.Context, coterieID string) (string, error) {
	var ownerID string
	err := s.db.WithContext(ctx).
		Table("coteries c").
		Select("s.owner_user_id").
		Joins("JOIN subscriptions s ON s.id = c.subscription_id").
		Where("c.id = ?", coterieID).
		Scan(&ownerID).Error
	return ownerID, err
}

// PendingByUserAndCoterie returns the user's live request for the
// coterie, or nil when absent.
func (s *Store) PendingByUserAndCoterie(ctx context.Context, coterieID, userID string) (*JoinRequest, error) {
	var r JoinRequest
	err := s.db.WithContext(ctx).
		First(&r, "coterie_id = ? AND user_id = ? AND status = ?", coterieID, userID, RequestPending).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// Create inserts the join request.
func (s *Store) Create(ctx context.Context, r *JoinRequest) error {
	return s.db.WithContext(ctx).Create(r).Error
}

// RequestByID returns the join request, or nil when absent.
func (s *Store) RequestByID(ctx context.Context, id string) (*JoinRequest, error) {
	var r JoinRequest
	err := s.db.WithContext(ctx).First(&r, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ListByCoterie returns the coterie's requests, newest first, narrowed
// by an optional status, joined with the requester's display name.
func (s *Store) ListByCoterie(ctx context.Context, coterieID, status string, page api.Page) ([]JoinRequestView, int64, error) {
	q := s.db.WithContext(ctx).
		Table("join_requests j").
		Joins("JOIN users u ON u.id = j.user_id").
		Where("j.coterie_id = ?", coterieID)
	if status != "" {
		q = q.Where("j.status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var views []JoinRequestView
	err := q.
		Select("j.*, u.username AS username").
		Order("j.created_at DESC, j.id ASC").
		Limit(page.Limit).
		Offset(page.Offset).
		Scan(&views).Error
	return views, total, err
}

// SetStatus moves a pending request to a terminal state and reports
// whether the row changed.
func (s *Store) SetStatus(ctx context.Context, id, status string) (bool, error) {
	res := s.db.WithContext(ctx).Model(&JoinRequest{}).
		Where("id = ? AND status = ?", id, RequestPending).
		Updates(map[string]any{"status": status, "decided_at": time.Now().UTC()})
	return res.RowsAffected > 0, res.Error
}
