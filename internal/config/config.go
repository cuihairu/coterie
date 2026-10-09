// Package config loads server configuration from the environment.
// Defaults match deployments/docker-compose.yml and .env.example.
package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/coterie/internal/ratelimit"
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

	PushVapidPublicKey  string // Web Push VAPID identity (D22); empty disables the push channel
	PushVapidPrivateKey string
	PushVapidSubject    string

	// Stripe channel credentials (D24). The secret key enables the
	// stripe method; the webhook secret verifies its confirmations.
	StripeSecretKey     string
	StripeWebhookSecret string
	StripeAPIBase       string

	// The marketplace gate's own webhook endpoint signing secret
	// (design D25); empty falls back to StripeWebhookSecret.
	StripeMarketplaceWebhookSecret string

	// Payment channels to register on top of the always-on manual
	// adapter (design §4.2), comma-separated; unknown names are skipped
	// with a warning at startup.
	PaymentMethods []string

	// Provider plugins to register on top of the always-empty registry
	// (design §5.1/§5.2), comma-separated; unknown names are skipped
	// with a warning at startup.
	ProviderPlugins []string

	// Cadence of the billing rollover scheduler (design D13); 0
	// disables the scheduler entirely. Parses Go duration strings
	// ("1h", "30s").
	AutoBillingInterval time.Duration

	// Public-endpoint rate limits per client address per minute
	// (design D19); 0 disables a class.
	RateLimitRegisterPerMin int
	RateLimitLoginPerMin    int
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

		PushVapidPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
		PushVapidPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
		PushVapidSubject:    os.Getenv("VAPID_SUBJECT"),

		StripeSecretKey:     os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
		StripeAPIBase:       envOr("STRIPE_API_BASE", "https://api.stripe.com"),

		StripeMarketplaceWebhookSecret: os.Getenv("STRIPE_MARKETPLACE_WEBHOOK_SECRET"),

		PaymentMethods: envList("PAYMENT_METHODS"),

		ProviderPlugins: envList("PROVIDER_PLUGINS"),

		AutoBillingInterval: envDuration("AUTO_BILLING_INTERVAL", time.Hour),

		RateLimitRegisterPerMin: envInt("RATE_LIMIT_REGISTER_PER_MIN", ratelimit.DefaultRegisterPerMin),
		RateLimitLoginPerMin:    envInt("RATE_LIMIT_LOGIN_PER_MIN", ratelimit.DefaultLoginPerMin),
	}
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
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

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
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
