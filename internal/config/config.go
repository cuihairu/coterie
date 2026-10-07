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
}

// Load reads configuration from environment variables, applying defaults.
func Load() Config {
	return Config{
		Port:           envOr("PORT", "8080"),
		DatabaseURL:    envOr("DATABASE_URL", "postgres://coterie:coterie@localhost:5432/coterie?sslmode=disable"),
		MigrationsDir:  envOr("MIGRATIONS_DIR", "migrations"),
		MigrateOnStart: envBool("MIGRATE_ON_START", true),
		LogLevel:       envLevel("LOG_LEVEL", slog.LevelInfo),
	}
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
