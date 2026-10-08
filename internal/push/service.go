// Package push registers browser push endpoints for Web Push delivery
// (design D22, FR-12). Registration is authenticated and self-scoped;
// the outbound side lives in the notification module's PushChannel.
package push

import (
	"context"
	"encoding/base64"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// Service registers and lists push subscriptions.
type Service struct {
	store *Store
}

// NewService builds a Service.
func NewService(db *gorm.DB) *Service {
	return &Service{store: NewStore(db)}
}

// Store exposes the underlying store for the notification module's
// channel adapter.
func (s *Service) Store() *Store { return s.store }

// Register records (or refreshes) the caller's push subscription.
// Endpoint must be https and the keys must decode as base64url — the
// channel encrypts with them verbatim, so a bad key would only fail
// later at delivery time.
func (s *Service) Register(ctx context.Context, actor *user.User, req RegisterRequest) (*Subscription, error) {
	req.Endpoint = strings.TrimSpace(req.Endpoint)
	u, err := url.Parse(req.Endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, api.Validation("invalid push subscription",
			api.Detail{Field: "endpoint", Message: "must be an https push service URL"})
	}
	if p256dh, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(req.Keys.P256dh)); err != nil ||
		len(p256dh) != 65 || p256dh[0] != 4 {
		return nil, api.Validation("invalid push subscription",
			api.Detail{Field: "keys.p256dh", Message: "must be a base64url uncompressed P-256 point (65 bytes)"})
	}
	if auth, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(req.Keys.Auth)); err != nil || len(auth) < 16 {
		return nil, api.Validation("invalid push subscription",
			api.Detail{Field: "keys.auth", Message: "must be a base64url secret of at least 16 bytes"})
	}
	sub := &Subscription{
		ID:       uuid.NewString(),
		UserID:   actor.ID,
		Endpoint: req.Endpoint,
		P256dh:   strings.TrimSpace(req.Keys.P256dh),
		Auth:     strings.TrimSpace(req.Keys.Auth),
	}
	if err := s.store.UpsertByEndpoint(ctx, sub); err != nil {
		return nil, err
	}
	// Upserts keep the original row id; return the stored state so the
	// response always echoes what is on disk.
	if stored, err := s.store.ByEndpoint(ctx, sub.Endpoint); err == nil && stored != nil {
		return stored, nil
	}
	return sub, nil
}

// List returns the caller's registered endpoints.
func (s *Service) List(ctx context.Context, actor *user.User) ([]Subscription, error) {
	return s.store.ListByUser(ctx, actor.ID)
}

// Delete removes one of the caller's subscriptions; unknown ids and
// other people's subscriptions are both 404.
func (s *Service) Delete(ctx context.Context, actor *user.User, id string) error {
	sub, err := s.store.ByID(ctx, id)
	if err != nil {
		return err
	}
	if sub == nil || sub.UserID != actor.ID {
		return api.NotFound("push subscription %s not found", id)
	}
	_, err = s.store.Delete(ctx, id)
	return err
}
