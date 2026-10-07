// Package config loads server configuration from the environment.
// Defaults match deployments/docker-compose.yml and .env.example.
package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Config holds the server runtime configuration.
type Config struct {
	Port           string
	DatabaseURL    string
	MigrationsDir  string
	MigrateOnStart bool
	LogLevel       slog.Level

	// Outbound notification channels (FR-12). A channel is enabled by
	// its primary setting; both default to disabled.
	SMTPHost      string
	SMTPPort      string
	SMTPUsername  string
	SMTPPassword  string
	SMTPFrom      string
	WebhookURL    string
	WebhookSecret string

	// Payment channels to register on top of the always-on manual
	// adapter (design §4.2), comma-separated; unknown names are skipped
	// with a warning at startup.
	PaymentMethods []string
}

// Load reads configuration from environment variables, applying defaults.
func Load() Config {
	return Config{
		Port:           envOr("PORT", "8080"),
		DatabaseURL:    envOr("DATABASE_URL", "postgres://coterie:coterie@localhost:5432/coterie?sslmode=disable"),
		MigrationsDir:  envOr("MIGRATIONS_DIR", "migrations"),
		MigrateOnStart: envBool("MIGRATE_ON_START", true),
		LogLevel:       envLevel("LOG_LEVEL", slog.LevelInfo),

		SMTPHost:      os.Getenv("SMTP_HOST"),
		SMTPPort:      envOr("SMTP_PORT", "587"),
		SMTPUsername:  os.Getenv("SMTP_USERNAME"),
		SMTPPassword:  os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:      os.Getenv("SMTP_FROM"),
		WebhookURL:    os.Getenv("WEBHOOK_URL"),
		WebhookSecret: os.Getenv("WEBHOOK_SECRET"),

		PaymentMethods: envList("PAYMENT_METHODS"),
	}
}

func envList(key string) []string {
	var out []string
	for _, part := range strings.Split(os.Getenv(key), ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func envLevel(key string, fallback slog.Level) slog.Level {
	switch strings.ToUpper(os.Getenv(key)) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return fallback
	}
}
