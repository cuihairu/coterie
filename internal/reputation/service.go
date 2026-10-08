package reputation

import (
	"context"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/pkg/api"
)

// Service derives reputation reports.
type Service struct {
	store *Store
}

// NewService builds a Service.
func NewService(db *gorm.DB) *Service {
	return &Service{store: NewStore(db)}
}

// For returns the user's reputation, mapping absence to 404.
func (s *Service) For(ctx context.Context, userID string) (*Report, error) {
	ok, err := s.store.UserExists(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, api.NotFound("user %s not found", userID)
	}
	return s.store.Aggregate(ctx, userID)
}

// Reports batch-derives reputation for the given user ids (D18 badge
// projection). Callers dedup; absent users map to empty reports.
func (s *Service) Reports(ctx context.Context, userIDs []string) (map[string]Report, error) {
	return s.store.Reports(ctx, userIDs)
}
