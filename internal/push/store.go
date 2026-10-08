package push

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// Store holds the GORM queries for push subscriptions.
type Store struct {
	db *gorm.DB
}

// NewStore builds a Store.
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// UpsertByEndpoint inserts the subscription, or refreshes the key
// material when the browser re-registers the same endpoint (keys rotate
// on resubscribe).
func (s *Store) UpsertByEndpoint(ctx context.Context, sub *Subscription) error {
	return s.db.WithContext(ctx).
		Exec(`INSERT INTO push_subscriptions (id, user_id, endpoint, p256dh, auth, created_at)
			VALUES (?, ?, ?, ?, ?, now())
			ON CONFLICT (endpoint) DO UPDATE SET user_id = EXCLUDED.user_id, p256dh = EXCLUDED.p256dh, auth = EXCLUDED.auth`,
			sub.ID, sub.UserID, sub.Endpoint, sub.P256dh, sub.Auth).Error
}

// ListByUser returns the user's registered endpoints, oldest first.
func (s *Store) ListByUser(ctx context.Context, userID string) ([]Subscription, error) {
	var out []Subscription
	err := s.db.WithContext(ctx).
		Order("created_at ASC, id ASC").
		Find(&out, "user_id = ?", userID).Error
	return out, err
}

// ByID returns the subscription, or nil when absent.
func (s *Store) ByID(ctx context.Context, id string) (*Subscription, error) {
	var sub Subscription
	err := s.db.WithContext(ctx).First(&sub, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// ByEndpoint returns the subscription registered for the endpoint, or
// nil when absent — upserts keep the original row id, so callers
// re-fetch by endpoint rather than trusting the inserted value.
func (s *Store) ByEndpoint(ctx context.Context, endpoint string) (*Subscription, error) {
	var sub Subscription
	err := s.db.WithContext(ctx).First(&sub, "endpoint = ?", endpoint).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// Delete removes the subscription row; the caller owns the ownership
// check. rows reports whether anything was deleted.
func (s *Store) Delete(ctx context.Context, id string) (bool, error) {
	res := s.db.WithContext(ctx).Delete(&Subscription{}, "id = ?", id)
	return res.RowsAffected > 0, res.Error
}

// DeleteByEndpoints prunes the given endpoints — the channel adapter
// calls it when the push service reports them gone (404/410).
func (s *Store) DeleteByEndpoints(ctx context.Context, endpoints []string) error {
	if len(endpoints) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).
		Delete(&Subscription{}, "endpoint IN ?", endpoints).Error
}
