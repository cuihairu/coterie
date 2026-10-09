// Command server runs the Coterie API server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cuihairu/coterie/internal/app"
	"github.com/cuihairu/coterie/internal/automation"
	"github.com/cuihairu/coterie/internal/billing"
	"github.com/cuihairu/coterie/internal/config"
	"github.com/cuihairu/coterie/internal/database"
	"github.com/cuihairu/coterie/internal/notification"
	"github.com/cuihairu/coterie/internal/payment"
	"github.com/cuihairu/coterie/internal/provider"
	"github.com/cuihairu/coterie/internal/push"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/providers/claude"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	if err := run(cfg, log); err != nil {
		log.Error("server exited", "error", err)
		os.Exit(1)
	}
}

// channels builds the outbound notification channels that have
// configuration present; unconfigured channels stay disabled (FR-12).
func channels(db *gorm.DB, cfg config.Config) []notification.Channel {
	var out []notification.Channel
	email := notification.NewEmailChannel(db, cfg.SMTPHost, cfg.SMTPPort,
		cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)
	if email.Enabled() {
		out = append(out, email)
	}
	webhook := notification.NewWebhookChannel(cfg.WebhookURL, cfg.WebhookSecret)
	if webhook.Enabled() {
		out = append(out, webhook)
	}
	pushCh := notification.NewPushChannel(push.NewService(db).Store(),
		cfg.PushVapidPublicKey, cfg.PushVapidPrivateKey, cfg.PushVapidSubject)
	if pushCh.Enabled() {
		out = append(out, pushCh)
	}
	return out
}

// paymentAdapters maps configured method names to the adapters the
// binary knows; the manual adapter is always registered (design §4.2).
// Unknown names are skipped with a warning so a typo doesn't kill the
// server.
func paymentAdapters(cfg config.Config, log *slog.Logger) []payment.Adapter {
	var out []payment.Adapter
	for _, name := range cfg.PaymentMethods {
		switch name {
		case payment.Sandbox{}.Name():
			out = append(out, payment.Sandbox{})
		case payment.Stripe{}.Name():
			if cfg.StripeSecretKey == "" {
				log.Warn("PAYMENT_METHODS lists stripe without STRIPE_SECRET_KEY, skipping", "method", name)
				continue
			}
			out = append(out, payment.Stripe{
				SecretKey:     cfg.StripeSecretKey,
				WebhookSecret: cfg.StripeWebhookSecret,
				APIBase:       cfg.StripeAPIBase,
			})
		default:
			log.Warn("unknown payment method in PAYMENT_METHODS, skipping", "method", name)
		}
	}
	return out
}

// marketplaceAdapters builds the channels the marketplace payment gate
// (D25) confirms with. Stripe gets the gate's own webhook secret when
// configured — Stripe issues one per endpoint — falling back to the
// payments secret so single-endpoint setups keep working. Manual never
// appears here: it settles through the owner's accept.
func marketplaceAdapters(cfg config.Config, log *slog.Logger) []payment.Adapter {
	var out []payment.Adapter
	for _, name := range cfg.PaymentMethods {
		if name != (payment.Stripe{}).Name() {
			continue
		}
		if cfg.StripeSecretKey == "" {
			log.Warn("PAYMENT_METHODS lists stripe without STRIPE_SECRET_KEY, skipping", "method", name)
			continue
		}
		whsec := cfg.StripeMarketplaceWebhookSecret
		if whsec == "" {
			whsec = cfg.StripeWebhookSecret
		}
		out = append(out, payment.Stripe{
			SecretKey:     cfg.StripeSecretKey,
			WebhookSecret: whsec,
			APIBase:       cfg.StripeAPIBase,
		})
	}
	return out
}

// providerPlugins maps configured plugin names to the implementations
// the binary knows (design §5.2); the registry starts empty, so without
// configuration every provider behaves Generic. Unknown names are
// skipped with a warning so a typo doesn't kill the server.
func providerPlugins(cfg config.Config, log *slog.Logger) []provider.Plugin {
	var out []provider.Plugin
	for _, name := range cfg.ProviderPlugins {
		switch name {
		case claude.Slug:
			out = append(out, claude.Plugin{})
		default:
			log.Warn("unknown provider plugin in PROVIDER_PLUGINS, skipping", "plugin", name)
		}
	}
	return out
}

func run(cfg config.Config, log *slog.Logger) error {
	// Normalize the process to UTC: pgx decodes timestamptz into
	// time.Local, so the host's local zone would otherwise leak into
	// API responses. The session-level pin (database.Open) is not
	// enough because decoding happens client-side.
	time.Local = time.UTC

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = database.Close(db) }()

	if cfg.MigrateOnStart {
		log.Info("applying migrations", "dir", cfg.MigrationsDir)
		if err := database.Migrate(cfg.DatabaseURL, os.DirFS(cfg.MigrationsDir)); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	// Platform admin bootstrap (design D26): ADMIN_EMAILS promotes the
	// instance operator, idempotently.
	if n, err := user.EnsureAdmins(context.Background(), db, cfg.AdminEmails); err != nil {
		return fmt.Errorf("admin bootstrap: %w", err)
	} else if n > 0 {
		log.Info("promoted platform admins", "count", n)
	}

	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: app.New(db, log,
			app.WithNotificationChannels(channels(db, cfg)...),
			app.WithPaymentAdapters(paymentAdapters(cfg, log)...),
			app.WithMarketplaceAdapters(marketplaceAdapters(cfg, log)...),
			app.WithProviderPlugins(providerPlugins(cfg, log)...),
			app.WithRateLimits(cfg.RateLimitRegisterPerMin, cfg.RateLimitLoginPerMin)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Billing rollover scheduler (design D13); AUTO_BILLING_INTERVAL=0
	// disables it. It shares the process and database, acting as each
	// subscription's owner through the billing service. The notifier is
	// rebuilt here because app.New keeps its own instance.
	if cfg.AutoBillingInterval > 0 {
		notifier := notification.NewServiceWithChannels(db, log, channels(db, cfg)...)
		sched := automation.NewScheduler(db, billing.NewService(db, notifier), notifier, log, cfg.AutoBillingInterval)
		go sched.Run(ctx)
		log.Info("billing rollover scheduler running", "interval", cfg.AutoBillingInterval)
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("coterie server listening", "addr", srv.Addr, "migrate_on_start", cfg.MigrateOnStart)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
