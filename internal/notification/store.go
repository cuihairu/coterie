package notification

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/pkg/api"
)

// Store holds the GORM queries for notifications.
type Store struct {
	db *gorm.DB
}

// NewStore builds a Store.
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// Create inserts one notification.
func (s *Store) Create(ctx context.Context, n *Notification) error {
	return s.db.WithContext(ctx).Create(n).Error
}

// List returns a user's notifications, newest first, optionally only
// the unread ones.
func (s *Store) List(ctx context.Context, userID string, unreadOnly bool, page api.Page) ([]Notification, int64, error) {
	q := s.db.WithContext(ctx).Model(&Notification{}).Where("user_id = ?", userID)
	if unreadOnly {
		q = q.Where("read_at IS NULL")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Notification
	err := q.Order("created_at DESC, id DESC").
		Limit(page.Limit).
		Offset(page.Offset).
		Find(&items).Error
	return items, total, err
}

// Get returns the notification, or nil when absent.
func (s *Store) Get(ctx context.Context, id string) (*Notification, error) {
	var n Notification
	err := s.db.WithContext(ctx).First(&n, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// MarkRead stamps read_at exactly once and reports whether this call
// changed the row.
func (s *Store) MarkRead(ctx context.Context, id string, at time.Time) (bool, error) {
	res := s.db.WithContext(ctx).Model(&Notification{}).
		Where("id = ? AND read_at IS NULL", id).
		Update("read_at", at)
	return res.RowsAffected > 0, res.Error
}
