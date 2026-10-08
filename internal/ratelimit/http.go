package ratelimit

import (
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/cuihairu/coterie/pkg/api"
)

// Guard wraps a handler with rate limiting for the public write
// endpoints (design D19): POST /api/v1/auth/register and
// POST /api/v1/auth/login, keyed by client address. Every other
// request passes through untouched; disabled classes as well.
func (l *Limiter) Guard() api.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			class, limited := classOf(r)
			if limited {
				retry, ok := l.Allow(class, clientIP(r))
				if !ok {
					w.Header().Set("Retry-After", ceilSeconds(retry))
					api.WriteError(w, api.TooManyRequests(
						"too many %s requests from this address; retry after %s",
						class, retry.Round(time.Second)))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// classOf maps a request onto its limit class; ok is false for
// requests outside the Phase 1 surface. The paths mirror the mux's
// exact-match patterns for the auth routes.
func classOf(r *http.Request) (string, bool) {
	if r.Method != http.MethodPost {
		return "", false
	}
	switch r.URL.Path {
	case "/api/v1/auth/register":
		return ClassRegister, true
	case "/api/v1/auth/login":
		return ClassLogin, true
	}
	return "", false
}

// clientIP extracts the host part of RemoteAddr. The X-Forwarded-For
// header is deliberately ignored (D19): trusting proxy headers needs
// a deployment topology decision first.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ceilSeconds renders a duration as a whole-second Retry-After value,
// never under-reporting.
func ceilSeconds(d time.Duration) string {
	s := (d + time.Second - 1) / time.Second
	return strconv.FormatInt(int64(s), 10)
}
