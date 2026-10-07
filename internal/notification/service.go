package notification

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// Service writes and lists notifications. Domain modules call Notify
// after their transaction commits; failures are logged, never fatal —
// a lost notification must not roll back a seat assignment.
type Service struct {
	store    *Store
	log      *slog.Logger
	channels []Channel
}

// NewService builds a Service.
func NewService(db *gorm.DB) *Service {
	return &Service{store: NewStore(db), log: slog.Default()}
}

// Notify records one notification for the user. entityType/entityId may
// be empty for system-wide messages.
func (s *Service) Notify(ctx context.Context, userID, typ, title, body, entityType, entityID string) {
	n := &Notification{
		ID:         uuid.NewString(),
		UserID:     userID,
		Type:       typ,
		Title:      title,
		Body:       body,
		EntityType: entityType,
		EntityID:   entityID,
		CreatedAt:  time.Now().UTC(),
	}
	if err := s.store.Create(ctx, n); err != nil {
		s.log.Error("write notification failed", "user_id", userID, "type", typ, "err", err)
		return
	}
	s.dispatchAsync(n)
}

// List returns the actor's notifications, optionally unread only.
func (s *Service) List(ctx context.Context, actor *user.User, unreadOnly bool, page api.Page) ([]Notification, int64, error) {
	return s.store.List(ctx, actor.ID, unreadOnly, page)
}

// MarkRead marks the actor's own notification as read; touching
// someone else's is a 404, not a 403 — existence is not disclosed.
func (s *Service) MarkRead(ctx context.Context, actor *user.User, id string) (*Notification, error) {
	n, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if n == nil || n.UserID != actor.ID {
		return nil, api.NotFound("notification %s not found", id)
	}
	if changed, err := s.store.MarkRead(ctx, n.ID, time.Now().UTC()); err != nil {
		return nil, err
	} else if changed || n.ReadAt == nil {
		updated, err := s.store.Get(ctx, n.ID)
		if err != nil {
			return nil, err
		}
		if updated != nil {
			return updated, nil
		}
	}
	return n, nil
}
