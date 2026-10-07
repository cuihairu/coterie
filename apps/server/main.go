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
)

func main() {
	cfg := config.Load()
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	if err := run(cfg, log); err != nil {
		log.Error("server exited", "error", err)
		os.Exit(1)
	}
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
		Addr:              ":" + cfg.Port,
		Handler:           app.New(db, log),
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
