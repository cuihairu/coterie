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
	"github.com/cuihairu/coterie/internal/config"
	"github.com/cuihairu/coterie/internal/database"
	"github.com/cuihairu/coterie/internal/notification"
	"github.com/cuihairu/coterie/internal/payment"
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
		default:
			log.Warn("unknown payment method in PAYMENT_METHODS, skipping", "method", name)
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
	defer database.Close(db)

	if cfg.MigrateOnStart {
		log.Info("applying migrations", "dir", cfg.MigrationsDir)
		if err := database.Migrate(cfg.DatabaseURL, os.DirFS(cfg.MigrationsDir)); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: app.New(db, log,
			app.WithNotificationChannels(channels(db, cfg)...),
			app.WithPaymentAdapters(paymentAdapters(cfg, log)...)),
		ReadHeaderTimeout: 5 * time.Second,
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
