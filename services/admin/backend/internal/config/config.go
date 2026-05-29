package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// AppEnv identifies the deployment environment.
type AppEnv string

// Known AppEnv values — the application refuses to start with any other value.
const (
	AppEnvLocal      AppEnv = "local"
	AppEnvTesting    AppEnv = "testing"
	AppEnvStaging    AppEnv = "staging"
	AppEnvProduction AppEnv = "production"
)

// validAppEnv reports whether e is a recognised AppEnv value.
func validAppEnv(e AppEnv) bool {
	switch e {
	case AppEnvLocal, AppEnvTesting, AppEnvStaging, AppEnvProduction:
		return true
	}
	return false
}

// Config holds all runtime configuration values loaded from environment variables.
type Config struct {
	DatabaseURL        string
	JWTSecret          string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	FrontendURL        string
	Port               string
	AppEnv             AppEnv
	CacheEnabled       bool
	CacheURL           string
	CacheTTL           time.Duration
	RequestLogEnabled  bool
	// Stripe (all optional — billing endpoints return 503 when StripeSecretKey is unset)
	StripeSecretKey     string
	StripeWebhookSecret string
	StripePriceIDPro    string
	// SMTP (all optional — password-reset emails are skipped and a server-side
	// warning is logged when SMTPHost is unset. The forgot-password endpoint
	// still returns the neutral 2xx response to avoid leaking the disabled
	// state to clients).
	SMTPHost      string
	SMTPPort      int
	SMTPUsername  string
	SMTPPassword  string
	SMTPFrom      string
	SMTPFromName  string
	SMTPForceIPv4 bool
}

// Load reads configuration from environment variables.
// It attempts to load a .env file first (silently ignored in production where it won't exist).
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		JWTSecret:           os.Getenv("JWT_SECRET"),
		GoogleClientID:      os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:  os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:   envOrDefault("GOOGLE_REDIRECT_URL", "http://localhost:8080/v1/auth/google/callback"),
		FrontendURL:         envOrDefault("FRONTEND_URL", "http://localhost:3000"),
		Port:                envOrDefault("PORT", "8080"),
		CacheEnabled:        os.Getenv("CACHE_ENABLED") == "true",
		CacheURL:            envOrDefault("CACHE_URL", "redis://localhost:6379"),
		RequestLogEnabled:   os.Getenv("REQUEST_LOG_ENABLED") != "false",
		StripeSecretKey:     os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
		StripePriceIDPro:    os.Getenv("STRIPE_PRO_PRICE_ID"),
		SMTPHost:            os.Getenv("SMTP_HOST"),
		SMTPUsername:        os.Getenv("SMTP_USERNAME"),
		SMTPPassword:        os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:            os.Getenv("SMTP_FROM"),
		SMTPFromName:        envOrDefault("SMTP_FROM_NAME", "Charity Chest"),
		SMTPForceIPv4:       os.Getenv("SMTP_FORCE_IPV4") != "false",
	}

	cacheTTL, err := parseDuration(os.Getenv("CACHE_TTL"), 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("invalid CACHE_TTL: %w", err)
	}
	cfg.CacheTTL = cacheTTL

	smtpPort, err := parsePort(os.Getenv("SMTP_PORT"), 587)
	if err != nil {
		return nil, fmt.Errorf("invalid SMTP_PORT: %w", err)
	}
	cfg.SMTPPort = smtpPort

	var missing []string

	// APP_ENV is required and must be one of the known values.
	rawAppEnv := os.Getenv("APP_ENV")
	if rawAppEnv == "" {
		missing = append(missing, "APP_ENV")
	} else {
		env := AppEnv(rawAppEnv)
		if !validAppEnv(env) {
			return nil, fmt.Errorf("invalid APP_ENV %q: must be one of local, testing, staging, production", rawAppEnv)
		}
		cfg.AppEnv = env
	}

	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if cfg.GoogleClientID == "" {
		missing = append(missing, "GOOGLE_CLIENT_ID")
	}
	if cfg.GoogleClientSecret == "" {
		missing = append(missing, "GOOGLE_CLIENT_SECRET")
	}

	// When Stripe is enabled (STRIPE_SECRET_KEY is set), all three vars are required.
	// A partial Stripe config would allow webhooks without signature verification.
	if cfg.StripeSecretKey != "" {
		if cfg.StripeWebhookSecret == "" {
			missing = append(missing, "STRIPE_WEBHOOK_SECRET")
		}
		if cfg.StripePriceIDPro == "" {
			missing = append(missing, "STRIPE_PRO_PRICE_ID")
		}
	}

	// When SMTP is enabled (SMTP_HOST is set), SMTP_FROM is required so the
	// recovery email has a sender address. SMTP_USERNAME and SMTP_PASSWORD are
	// an optional pair: both or neither (Mailpit and many internal relays
	// accept unauthenticated submissions). Empty strings are treated as unset.
	if cfg.SMTPHost != "" {
		if cfg.SMTPFrom == "" {
			missing = append(missing, "SMTP_FROM")
		}
		usernameSet := cfg.SMTPUsername != ""
		passwordSet := cfg.SMTPPassword != ""
		if usernameSet != passwordSet {
			if !usernameSet {
				missing = append(missing, "SMTP_USERNAME")
			}
			if !passwordSet {
				missing = append(missing, "SMTP_PASSWORD")
			}
		}
	}

	if len(missing) > 0 {
		return nil, errors.New("missing required environment variables: " + strings.Join(missing, ", "))
	}

	return cfg, nil
}

// parsePort parses a TCP port from s; returns def when s is empty. Rejects
// values outside the valid TCP port range (1–65535).
func parsePort(s string, def int) (int, error) {
	if s == "" {
		return def, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if v < 1 || v > 65535 {
		return 0, fmt.Errorf("port out of range: %d", v)
	}
	return v, nil
}

// envOrDefault returns the value of the environment variable key, or def if it is unset.
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// parseDuration parses a duration string; returns def when s is empty.
func parseDuration(s string, def time.Duration) (time.Duration, error) {
	if s == "" {
		return def, nil
	}
	return time.ParseDuration(s)
}
