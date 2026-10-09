package audit

import (
	"context"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// Service exposes the owner-only audit reads.
type Service struct {
	store *Store
}

// NewService builds a Service.
func NewService(db *gorm.DB) *Service {
	return &Service{store: NewStore(db)}
}

// ListForSubscription returns the subscription's audit trail. Only the
// subscription owner may read it.
func (s *Service) ListForSubscription(ctx context.Context, actor *user.User, subscriptionID, action string, page api.Page) ([]Entry, int64, error) {
	if action != "" && !Actions[action] {
		return nil, 0, api.Validation("invalid filter",
			api.Detail{Field: "action", Message: "unknown audit action"})
	}
	owner, err := s.store.SubscriptionOwner(ctx, subscriptionID)
	if err != nil {
		return nil, 0, err
	}
	if owner == "" {
		return nil, 0, api.NotFound("subscription %s not found", subscriptionID)
	}
	if owner != actor.ID {
		return nil, 0, api.Forbidden("only the subscription owner may read its audit log")
	}
	return s.store.ListBySubscription(ctx, subscriptionID, action, page)
}

// ListForCoterie returns the coterie's audit trail. Only the owner of
// the subscription the coterie belongs to may read it.
func (s *Service) ListForCoterie(ctx context.Context, actor *user.User, coterieID, action string, page api.Page) ([]Entry, int64, error) {
	if action != "" && !Actions[action] {
		return nil, 0, api.Validation("invalid filter",
			api.Detail{Field: "action", Message: "unknown audit action"})
	}
	owner, err := s.store.CoterieOwner(ctx, coterieID)
	if err != nil {
		return nil, 0, err
	}
	if owner == "" {
		return nil, 0, api.NotFound("coterie %s not found", coterieID)
	}
	if owner != actor.ID {
		return nil, 0, api.Forbidden("only the subscription owner may read its audit log")
	}
	return s.store.ListByCoterie(ctx, coterieID, action, page)
}

// ListAll is the platform admin's read (design D27): the whole ledger
// across subscriptions and coteries. The admin gate itself lives on
// the route middleware.
func (s *Service) ListAll(ctx context.Context, action, actorID, coterieID, subscriptionID string, page api.Page) ([]Entry, int64, error) {
	if action != "" && !Actions[action] {
		return nil, 0, api.Validation("invalid filter",
			api.Detail{Field: "action", Message: "unknown audit action"})
	}
	return s.store.ListAll(ctx, action, actorID, coterieID, subscriptionID, page)
}
