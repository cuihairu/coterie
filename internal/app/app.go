// Package app assembles the HTTP handler: healthz plus every module's
// routes, wrapped in shared middleware. It lives in its own package so
// tests can exercise the full mux without importing package main.
package app

import (
	"log/slog"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/product"
	"github.com/cuihairu/coterie/internal/provider"
	"github.com/cuihairu/coterie/internal/subscription"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// New assembles the server handler. db must not be nil in real runs;
// tests may pass nil to exercise healthz only.
func New(db *gorm.DB, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		api.WriteJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	if db != nil {
		user.RegisterRoutes(mux, user.NewService(db))
		provider.RegisterRoutes(mux, provider.NewService(db))
		product.RegisterRoutes(mux, product.NewService(db))
		subscription.RegisterRoutes(mux, subscription.NewService(db))
	}

	// Catch-all so unmatched paths return the JSON error envelope
	// instead of ServeMux's plain-text 404.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		api.WriteError(w, api.NotFound("route %s %s not found", r.Method, r.URL.Path))
	})

	return api.Chain(mux,
		api.Recover(log),
		api.RequestLogger(log),
	)
}
