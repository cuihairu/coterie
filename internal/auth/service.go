package auth

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"

	"github.com/google/uuid"
)

// sessionTTL bounds how long an idle bearer token stays valid.
const sessionTTL = 30 * 24 * time.Hour

// Service carries the authentication rules: registration, sessions,
// and bearer-token verification.
type Service struct {
	store *Store
	users *user.Service
	log   *slog.Logger
}

// NewService builds a Service.
func NewService(db *gorm.DB) *Service {
	return &Service{store: NewStore(db), users: user.NewService(db), log: slog.Default()}
}

// Register creates the user (via the user module) and issues a session.
// Password rules live in user.Service so both /users and /auth/register
// validate identically.
func (s *Service) Register(ctx context.Context, req user.CreateUserRequest) (*AuthResponse, error) {
	if req.Password == "" {
		return nil, api.Validation("invalid credentials",
			api.Detail{Field: "password", Message: "required"})
	}
	u, err := s.users.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	return s.issueSession(ctx, u)
}

// Login verifies email + password and issues a session. Missing user,
// passwordless user, and wrong password share one message to avoid
// account enumeration.
func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	req.Email = strings.TrimSpace(req.Email)
	u, err := s.store.UserByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if u == nil || u.PasswordHash == nil ||
		bcrypt.CompareHashAndPassword([]byte(*u.PasswordHash), []byte(req.Password)) != nil {
		return nil, api.Unauthorized("invalid email or password")
	}
	return s.issueSession(ctx, u)
}

// Logout revokes the session behind token. It is idempotent.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.DeleteSessionByTokenHash(ctx, HashToken(token))
}

// UserByToken resolves a bearer token to its user, rejecting unknown,
// revoked, and expired sessions with 401. An expired row is deleted on
// sight so dead sessions do not accumulate.
func (s *Service) UserByToken(ctx context.Context, token string) (*user.User, error) {
	sess, err := s.store.SessionByTokenHash(ctx, HashToken(token))
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, api.Unauthorized("invalid or expired session")
	}
	if time.Now().UTC().After(sess.ExpiresAt) {
		// Hygiene only — the 401 is already decided; a failed delete
		// must not mask it.
		_ = s.store.DeleteSessionByTokenHash(ctx, sess.TokenHash)
		return nil, api.Unauthorized("invalid or expired session")
	}
	u, err := s.users.Get(ctx, sess.UserID)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Service) issueSession(ctx context.Context, u *user.User) (*AuthResponse, error) {
	// Bounding pass: drop this user's expired sessions before adding a
	// fresh one, so returning users do not accumulate dead rows.
	if err := s.store.DeleteExpiredSessionsForUser(ctx, u.ID); err != nil {
		return nil, err
	}
	token, err := NewToken()
	if err != nil {
		return nil, err
	}
	sess := &Session{
		ID:        uuid.NewString(),
		UserID:    u.ID,
		TokenHash: HashToken(token),
		ExpiresAt: time.Now().UTC().Add(sessionTTL),
	}
	if err := s.store.CreateSession(ctx, sess); err != nil {
		return nil, err
	}
	return &AuthResponse{Token: token, User: u}, nil
}
