package config

import (
	"log/slog"
	"testing"
	"time"

	"github.com/cuihairu/coterie/internal/ratelimit"
)

func TestLoadDefaults(t *testing.T) {
	// Clear every key Load reads so the defaults hold regardless of the
	// environment the suite runs in.
	for _, key := range []string{
		"PORT", "DATABASE_URL", "MIGRATIONS_DIR", "MIGRATE_ON_START", "LOG_LEVEL",
		"SMTP_PORT", "VAPID_PUBLIC_KEY", "STRIPE_API_BASE", "AUTO_BILLING_INTERVAL",
		"RATE_LIMIT_REGISTER_PER_MIN", "RATE_LIMIT_LOGIN_PER_MIN",
		"PAYMENT_METHODS", "PROVIDER_PLUGINS", "ADMIN_EMAILS",
	} {
		t.Setenv(key, "")
	}
	c := Load()

	if c.Port != "8080" {
		t.Errorf("Port = %q, want 8080", c.Port)
	}
	if c.MigrationsDir != "migrations" {
		t.Errorf("MigrationsDir = %q, want migrations", c.MigrationsDir)
	}
	if !c.MigrateOnStart {
		t.Error("MigrateOnStart = false, want true")
	}
	if c.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want Info", c.LogLevel)
	}
	if c.SMTPPort != "587" {
		t.Errorf("SMTPPort = %q, want 587", c.SMTPPort)
	}
	if c.StripeAPIBase != "https://api.stripe.com" {
		t.Errorf("StripeAPIBase = %q, want the production origin", c.StripeAPIBase)
	}
	if c.AutoBillingInterval != time.Hour {
		t.Errorf("AutoBillingInterval = %v, want 1h", c.AutoBillingInterval)
	}
	if c.RateLimitRegisterPerMin != ratelimit.DefaultRegisterPerMin {
		t.Errorf("RateLimitRegisterPerMin = %d, want %d",
			c.RateLimitRegisterPerMin, ratelimit.DefaultRegisterPerMin)
	}
	if c.RateLimitLoginPerMin != ratelimit.DefaultLoginPerMin {
		t.Errorf("RateLimitLoginPerMin = %d, want %d",
			c.RateLimitLoginPerMin, ratelimit.DefaultLoginPerMin)
	}
	if c.PaymentMethods != nil || c.AdminEmails != nil {
		t.Errorf("list settings = %v / %v, want nil when unset", c.PaymentMethods, c.AdminEmails)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("MIGRATE_ON_START", "false")
	t.Setenv("LOG_LEVEL", "Error")
	t.Setenv("AUTO_BILLING_INTERVAL", "30s")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("VAPID_PUBLIC_KEY", "pub")
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_x")
	t.Setenv("STRIPE_MARKETPLACE_WEBHOOK_SECRET", "whsec_gate")
	t.Setenv("ADMIN_EMAILS", " ops@example.com , admin@example.com ,,")
	t.Setenv("PAYMENT_METHODS", "sandbox")
	c := Load()

	if c.Port != "9090" {
		t.Errorf("Port = %q, want 9090", c.Port)
	}
	if c.MigrateOnStart {
		t.Error("MigrateOnStart = true, want false")
	}
	if c.LogLevel != slog.LevelError {
		t.Errorf("LogLevel = %v, want Error (case-insensitive)", c.LogLevel)
	}
	if c.AutoBillingInterval != 30*time.Second {
		t.Errorf("AutoBillingInterval = %v, want 30s", c.AutoBillingInterval)
	}
	if c.SMTPHost != "smtp.example.com" || c.PushVapidPublicKey != "pub" || c.StripeSecretKey != "sk_test_x" {
		t.Error("channel credentials not read from the environment")
	}
	if c.StripeMarketplaceWebhookSecret != "whsec_gate" {
		t.Errorf("gate secret = %q, want whsec_gate", c.StripeMarketplaceWebhookSecret)
	}
	want := []string{"ops@example.com", "admin@example.com"}
	if len(c.AdminEmails) != 2 || c.AdminEmails[0] != want[0] || c.AdminEmails[1] != want[1] {
		t.Errorf("AdminEmails = %v, want %v (trimmed, blanks dropped)", c.AdminEmails, want)
	}
	if len(c.PaymentMethods) != 1 || c.PaymentMethods[0] != "sandbox" {
		t.Errorf("PaymentMethods = %v, want [sandbox]", c.PaymentMethods)
	}
}

func TestLoadMalformedValuesFallBack(t *testing.T) {
	t.Setenv("LOG_LEVEL", "verbose")
	t.Setenv("AUTO_BILLING_INTERVAL", "soon")
	t.Setenv("MIGRATE_ON_START", "yes please")
	t.Setenv("RATE_LIMIT_REGISTER_PER_MIN", "many")
	t.Setenv("RATE_LIMIT_LOGIN_PER_MIN", "0")
	c := Load()

	if c.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want the Info fallback", c.LogLevel)
	}
	if c.AutoBillingInterval != time.Hour {
		t.Errorf("AutoBillingInterval = %v, want the 1h fallback", c.AutoBillingInterval)
	}
	if !c.MigrateOnStart {
		t.Error("MigrateOnStart = false, want the true fallback")
	}
	if c.RateLimitRegisterPerMin != ratelimit.DefaultRegisterPerMin {
		t.Errorf("RateLimitRegisterPerMin = %d, want the %d fallback",
			c.RateLimitRegisterPerMin, ratelimit.DefaultRegisterPerMin)
	}
	if c.RateLimitLoginPerMin != 0 {
		t.Errorf("RateLimitLoginPerMin = %d, want 0 (explicit disable)", c.RateLimitLoginPerMin)
	}
}
