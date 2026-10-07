// Package app assembles the HTTP handler: healthz plus every module's
// routes, wrapped in shared middleware. It lives in its own package so
// tests can exercise the full mux without importing package main.
package app

import (
	"log/slog"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/auth"
	"github.com/cuihairu/coterie/internal/billing"
	"github.com/cuihairu/coterie/internal/coterie"
	"github.com/cuihairu/coterie/internal/dispute"
	"github.com/cuihairu/coterie/internal/marketplace"
	"github.com/cuihairu/coterie/internal/notification"
	"github.com/cuihairu/coterie/internal/payment"
	"github.com/cuihairu/coterie/internal/product"
	"github.com/cuihairu/coterie/internal/provider"
	"github.com/cuihairu/coterie/internal/reputation"
	"github.com/cuihairu/coterie/internal/seat"
	"github.com/cuihairu/coterie/internal/subscription"
	"github.com/cuihairu/coterie/internal/usage"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// Option customizes the assembled handler.
type Option func(*options)

type options struct {
	notificationChannels []notification.Channel
	providerPlugins      []provider.Plugin
	paymentAdapters      []payment.Adapter
}

// WithNotificationChannels registers outbound delivery channels (FR-12);
// production wires them from config in apps/server.
func WithNotificationChannels(channels ...notification.Channel) Option {
	return func(o *options) { o.notificationChannels = append(o.notificationChannels, channels...) }
}

// WithProviderPlugins registers provider plugins (design §5.1);
// production wires them in apps/server as real providers arrive.
func WithProviderPlugins(plugins ...provider.Plugin) Option {
	return func(o *options) { o.providerPlugins = append(o.providerPlugins, plugins...) }
}

// WithPaymentAdapters registers extra payment channels on top of the
// always-present manual adapter (design §4.2); production wires them
// from config in apps/server.
func WithPaymentAdapters(adapters ...payment.Adapter) Option {
	return func(o *options) { o.paymentAdapters = append(o.paymentAdapters, adapters...) }
}

// New assembles the server handler. Every module route sits behind the
// auth middleware; only healthz, register/login, the public catalog
// reads, and the marketplace directory are open. db must not be nil in
// real runs; tests may pass nil to exercise healthz only.
func New(db *gorm.DB, log *slog.Logger, opts ...Option) http.Handler {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		api.WriteJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	if db != nil {
		authSvc := auth.NewService(db)
		requireUser := authSvc.RequireUser()
		notifier := notification.NewServiceWithChannels(db, log, o.notificationChannels...)

		auth.RegisterRoutes(mux, authSvc)
		user.RegisterRoutes(mux, user.NewService(db), requireUser)
		pluginRegistry := provider.NewRegistry(o.providerPlugins...)
		provider.RegisterRoutes(mux, provider.NewService(db), requireUser)
		product.RegisterRoutes(mux, product.NewService(db), requireUser)
		subscription.RegisterRoutes(mux, subscription.NewServiceWithPlugins(db, pluginRegistry), requireUser)
		seatSvc := seat.NewService(db, notifier)
		seat.RegisterRoutes(mux, seatSvc, requireUser)
		coterieSvc := coterie.NewService(db, seatSvc, notifier, pluginRegistry)
		coterie.RegisterRoutes(mux, coterieSvc, requireUser)
		marketplace.RegisterRoutes(mux, marketplace.NewService(db, coterieSvc, notifier), requireUser)
		billing.RegisterRoutes(mux, billing.NewService(db, notifier), requireUser)
		dispute.RegisterRoutes(mux, dispute.NewService(db, notifier), requireUser)
		reputation.RegisterRoutes(mux, reputation.NewService(db), requireUser)
		// Manual stays registered no matter what's configured on top.
		paymentAdapters := append([]payment.Adapter{payment.Manual{}}, o.paymentAdapters...)
		payment.RegisterRoutes(mux, payment.NewServiceWithAdapters(db, notifier, paymentAdapters...), requireUser)
		usage.RegisterRoutes(mux, usage.NewServiceWithPlugins(db, pluginRegistry), requireUser)
		notification.RegisterRoutes(mux, notifier, requireUser)
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
