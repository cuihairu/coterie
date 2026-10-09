package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

type contextKey struct{}

// UserFrom returns the authenticated user stored by RequireUser.
func UserFrom(ctx context.Context) (*user.User, bool) {
	u, ok := ctx.Value(contextKey{}).(*user.User)
	return u, ok
}

// RequireUser enforces a valid bearer session on the wrapped routes
// and stores the authenticated user in the request context.
func (s *Service) RequireUser() api.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" {
				api.WriteError(w, api.Unauthorized("missing bearer token"))
				return
			}
			u, err := s.UserByToken(r.Context(), token)
			if err != nil {
				api.WriteError(w, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, u)))
		})
	}
}

// RequireAdmin builds on RequireUser's session check and additionally
// demands the platform admin role (design D26). It wraps a RequireUser
// middleware — pass the result of RequireUser() to keep the session
// enforcement in one place.
func RequireAdmin(next api.Middleware) api.Middleware {
	return func(inner http.Handler) http.Handler {
		return next(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := UserFrom(r.Context())
			if !ok || u.Role != user.RoleAdmin {
				api.WriteError(w, api.Forbidden("platform admin role required"))
				return
			}
			inner.ServeHTTP(w, r)
		}))
	}
}

// bearerToken extracts the token from an Authorization: Bearer header.
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}
