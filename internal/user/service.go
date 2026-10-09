package user

import (
	"context"
	"log/slog"
	"strings"

	"golang.org/x/crypto/bcrypt"
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
	if req.Password != "" {
		if len(req.Password) < 8 {
			details = append(details, api.Detail{Field: "password", Message: "must be at least 8 characters"})
		} else if len(req.Password) > 72 {
			details = append(details, api.Detail{Field: "password", Message: "must be at most 72 characters"})
		}
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
		Role:     RoleUser,
	}
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		hashStr := string(hash)
		u.PasswordHash = &hashStr
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

// EnsureAdmins promotes the given emails (as configured via
// ADMIN_EMAILS) to the platform admin role — the instance operator's
// bootstrap (design D26). Addresses are normalized here so the match
// against lower(email) is case-insensitive; blank entries are skipped.
// Idempotent; returns the number of rows it flipped.
func EnsureAdmins(ctx context.Context, db *gorm.DB, emails []string) (int64, error) {
	wanted := make([]string, 0, len(emails))
	for _, e := range emails {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			wanted = append(wanted, e)
		}
	}
	if len(wanted) == 0 {
		return 0, nil
	}
	res := db.WithContext(ctx).Exec(
		`UPDATE users SET role = ?, updated_at = now()
		 WHERE role <> ? AND lower(email) IN ?`,
		RoleAdmin, RoleAdmin, wanted)
	return res.RowsAffected, res.Error
}
