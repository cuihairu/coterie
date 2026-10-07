package user

import (
	"context"
	"log/slog"
	"strings"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/database"
	"github.com/cuihairu/coterie/pkg/api"

	"github.com/google/uuid"
)

// Service carries the user business rules on top of the store.
type Service struct {
	store *Store
	log   *slog.Logger
}

// NewService builds a Service. The logger is only used for internal
// errors; pass slog.Default() when in doubt.
func NewService(db *gorm.DB) *Service {
	return &Service{store: NewStore(db), log: slog.Default()}
}

// Create validates and inserts a user.
func (s *Service) Create(ctx context.Context, req CreateUserRequest) (*User, error) {
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	var details []api.Detail
	if req.Username == "" {
		details = append(details, api.Detail{Field: "username", Message: "required"})
	} else if len(req.Username) > 64 {
		details = append(details, api.Detail{Field: "username", Message: "must be at most 64 characters"})
	}
	if req.Email == "" {
		details = append(details, api.Detail{Field: "email", Message: "required"})
	} else if len(req.Email) > 255 || !strings.Contains(req.Email, "@") {
		details = append(details, api.Detail{Field: "email", Message: "must be a valid email address"})
	}
	if len(req.Locale) > 35 {
		details = append(details, api.Detail{Field: "locale", Message: "must be at most 35 characters"})
	}
	if len(req.Timezone) > 64 {
		details = append(details, api.Detail{Field: "timezone", Message: "must be at most 64 characters"})
	}
	if len(details) > 0 {
		return nil, api.Validation("invalid user", details...)
	}

	if req.Locale == "" {
		req.Locale = DefaultLocale
	}
	if req.Timezone == "" {
		req.Timezone = DefaultTimezone
	}

	u := &User{
		ID:       uuid.NewString(),
		Username: req.Username,
		Email:    req.Email,
		Locale:   req.Locale,
		Timezone: req.Timezone,
		Status:   StatusActive,
	}
	if err := s.store.Create(ctx, u); err != nil {
		if database.IsUniqueViolation(err) {
			return nil, api.Conflict("username or email already exists")
		}
		return nil, err
	}
	return u, nil
}

// Get returns the user, mapping absence to 404.
func (s *Service) Get(ctx context.Context, id string) (*User, error) {
	u, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, api.NotFound("user %s not found", id)
	}
	return u, nil
}

// List returns a page of users with the total count.
func (s *Service) List(ctx context.Context, page api.Page) ([]User, int64, error) {
	return s.store.List(ctx, page)
}
