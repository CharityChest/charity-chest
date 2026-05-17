package config_test

import (
	"strings"
	"testing"
	"time"

	"charity-chest/internal/config"
)

// allRequired holds a valid set of all required env vars.
var allRequired = map[string]string{
	"DATABASE_URL":         "postgres://localhost/testdb",
	"JWT_SECRET":           "supersecret",
	"GOOGLE_CLIENT_ID":     "client-id",
	"GOOGLE_CLIENT_SECRET": "client-secret",
	"APP_ENV":              "local",
}

func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func TestLoad_Success(t *testing.T) {
	setEnv(t, allRequired)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DatabaseURL != allRequired["DATABASE_URL"] {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.JWTSecret != allRequired["JWT_SECRET"] {
		t.Errorf("JWTSecret = %q", cfg.JWTSecret)
	}
	if cfg.GoogleClientID != allRequired["GOOGLE_CLIENT_ID"] {
		t.Errorf("GoogleClientID = %q", cfg.GoogleClientID)
	}
	if cfg.GoogleClientSecret != allRequired["GOOGLE_CLIENT_SECRET"] {
		t.Errorf("GoogleClientSecret = %q", cfg.GoogleClientSecret)
	}
}

// TestLoad_MissingEachRequired verifies that omitting any single required variable
// returns an error that names the missing variable.
func TestLoad_MissingEachRequired(t *testing.T) {
	for key := range allRequired {
		t.Run(key, func(t *testing.T) {
			setEnv(t, allRequired)
			t.Setenv(key, "") // blank out the variable under test

			_, err := config.Load()
			if err == nil {
				t.Fatalf("expected error when %s is empty", key)
			}
			if !strings.Contains(err.Error(), key) {
				t.Errorf("error %q does not mention missing variable %s", err.Error(), key)
			}
		})
	}
}

func TestLoad_DefaultPort(t *testing.T) {
	setEnv(t, allRequired)
	t.Setenv("PORT", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want default 8080", cfg.Port)
	}
}

func TestLoad_CustomPort(t *testing.T) {
	setEnv(t, allRequired)
	t.Setenv("PORT", "9090")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want 9090", cfg.Port)
	}
}

func TestLoad_DefaultGoogleRedirectURL(t *testing.T) {
	setEnv(t, allRequired)
	t.Setenv("GOOGLE_REDIRECT_URL", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	const want = "http://localhost:8080/v1/auth/google/callback"
	if cfg.GoogleRedirectURL != want {
		t.Errorf("GoogleRedirectURL = %q, want %q", cfg.GoogleRedirectURL, want)
	}
}

func TestLoad_CustomGoogleRedirectURL(t *testing.T) {
	setEnv(t, allRequired)
	const custom = "https://myapp.com/auth/google/callback"
	t.Setenv("GOOGLE_REDIRECT_URL", custom)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GoogleRedirectURL != custom {
		t.Errorf("GoogleRedirectURL = %q, want %q", cfg.GoogleRedirectURL, custom)
	}
}

func TestLoad_CacheDisabledByDefault(t *testing.T) {
	setEnv(t, allRequired)
	t.Setenv("CACHE_ENABLED", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CacheEnabled {
		t.Error("CacheEnabled should default to false")
	}
}

func TestLoad_CacheEnabled(t *testing.T) {
	setEnv(t, allRequired)
	t.Setenv("CACHE_ENABLED", "true")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.CacheEnabled {
		t.Error("CacheEnabled should be true")
	}
}

func TestLoad_CacheTTL_Default(t *testing.T) {
	setEnv(t, allRequired)
	t.Setenv("CACHE_TTL", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CacheTTL != 5*time.Minute {
		t.Errorf("CacheTTL = %v, want 5m", cfg.CacheTTL)
	}
}

func TestLoad_CacheTTL_Custom(t *testing.T) {
	setEnv(t, allRequired)
	t.Setenv("CACHE_TTL", "30s")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CacheTTL != 30*time.Second {
		t.Errorf("CacheTTL = %v, want 30s", cfg.CacheTTL)
	}
}

func TestLoad_CacheTTL_InvalidReturnsError(t *testing.T) {
	setEnv(t, allRequired)
	t.Setenv("CACHE_TTL", "not-a-duration")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid CACHE_TTL, got nil")
	}
	if !strings.Contains(err.Error(), "CACHE_TTL") {
		t.Errorf("error %q does not mention CACHE_TTL", err.Error())
	}
}

func TestLoad_CacheURL_Default(t *testing.T) {
	setEnv(t, allRequired)
	t.Setenv("CACHE_URL", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CacheURL != "redis://localhost:6379" {
		t.Errorf("CacheURL = %q, want redis://localhost:6379", cfg.CacheURL)
	}
}

// --- Request log ---

func TestLoad_RequestLogEnabledByDefault(t *testing.T) {
	setEnv(t, allRequired)
	t.Setenv("REQUEST_LOG_ENABLED", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.RequestLogEnabled {
		t.Error("RequestLogEnabled should default to true")
	}
}

func TestLoad_RequestLogDisabled(t *testing.T) {
	setEnv(t, allRequired)
	t.Setenv("REQUEST_LOG_ENABLED", "false")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RequestLogEnabled {
		t.Error("RequestLogEnabled should be false when REQUEST_LOG_ENABLED=false")
	}
}

// --- Stripe validation ---

func TestLoad_StripeDisabled_NoValidation(t *testing.T) {
	// When STRIPE_SECRET_KEY is absent, no Stripe vars are required.
	setEnv(t, allRequired)
	t.Setenv("STRIPE_SECRET_KEY", "")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "")
	t.Setenv("STRIPE_PRO_PRICE_ID", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error with Stripe disabled: %v", err)
	}
	if cfg.StripeSecretKey != "" {
		t.Error("expected empty StripeSecretKey")
	}
}

func TestLoad_StripeFullyConfigured_Success(t *testing.T) {
	setEnv(t, allRequired)
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_xxx")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_xxx")
	t.Setenv("STRIPE_PRO_PRICE_ID", "price_xxx")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.StripeSecretKey != "sk_test_xxx" {
		t.Errorf("StripeSecretKey = %q", cfg.StripeSecretKey)
	}
	if cfg.StripeWebhookSecret != "whsec_xxx" {
		t.Errorf("StripeWebhookSecret = %q", cfg.StripeWebhookSecret)
	}
	if cfg.StripePriceIDPro != "price_xxx" {
		t.Errorf("StripePriceIDPro = %q", cfg.StripePriceIDPro)
	}
}

func TestLoad_StripeMissingWebhookSecret_ReturnsError(t *testing.T) {
	setEnv(t, allRequired)
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_xxx")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "")
	t.Setenv("STRIPE_PRO_PRICE_ID", "price_xxx")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when STRIPE_WEBHOOK_SECRET is missing")
	}
	if !strings.Contains(err.Error(), "STRIPE_WEBHOOK_SECRET") {
		t.Errorf("error %q does not mention STRIPE_WEBHOOK_SECRET", err.Error())
	}
}

func TestLoad_StripeMissingPriceID_ReturnsError(t *testing.T) {
	setEnv(t, allRequired)
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_xxx")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_xxx")
	t.Setenv("STRIPE_PRO_PRICE_ID", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when STRIPE_PRO_PRICE_ID is missing")
	}
	if !strings.Contains(err.Error(), "STRIPE_PRO_PRICE_ID") {
		t.Errorf("error %q does not mention STRIPE_PRO_PRICE_ID", err.Error())
	}
}

func TestLoad_StripeMissingBothCompanions_ErrorMentionsBoth(t *testing.T) {
	setEnv(t, allRequired)
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_xxx")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "")
	t.Setenv("STRIPE_PRO_PRICE_ID", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when both companion Stripe vars are missing")
	}
	if !strings.Contains(err.Error(), "STRIPE_WEBHOOK_SECRET") {
		t.Errorf("error %q does not mention STRIPE_WEBHOOK_SECRET", err.Error())
	}
	if !strings.Contains(err.Error(), "STRIPE_PRO_PRICE_ID") {
		t.Errorf("error %q does not mention STRIPE_PRO_PRICE_ID", err.Error())
	}
}

// --- AppEnv ---

func TestLoad_AppEnv_EachValidValue(t *testing.T) {
	cases := []config.AppEnv{
		config.AppEnvLocal,
		config.AppEnvTesting,
		config.AppEnvStaging,
		config.AppEnvProduction,
	}
	for _, want := range cases {
		t.Run(string(want), func(t *testing.T) {
			setEnv(t, allRequired)
			t.Setenv("APP_ENV", string(want))
			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("unexpected error for APP_ENV=%q: %v", want, err)
			}
			if cfg.AppEnv != want {
				t.Errorf("AppEnv = %q, want %q", cfg.AppEnv, want)
			}
		})
	}
}

func TestLoad_AppEnv_UnsetReturnsError(t *testing.T) {
	setEnv(t, allRequired)
	t.Setenv("APP_ENV", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when APP_ENV is empty, got nil")
	}
	if !strings.Contains(err.Error(), "APP_ENV") {
		t.Errorf("error %q does not mention APP_ENV", err.Error())
	}
}

func TestLoad_AppEnv_InvalidValueReturnsError(t *testing.T) {
	setEnv(t, allRequired)
	t.Setenv("APP_ENV", "dev")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid APP_ENV, got nil")
	}
	if !strings.Contains(err.Error(), "APP_ENV") {
		t.Errorf("error %q does not mention APP_ENV", err.Error())
	}
	if !strings.Contains(err.Error(), "dev") {
		t.Errorf("error %q does not mention the invalid value", err.Error())
	}
}

// --- SMTP validation ---

// clearSMTP unsets every SMTP-related env var so tests start from a clean slate.
func clearSMTP(t *testing.T) {
	t.Helper()
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("SMTP_USERNAME", "")
	t.Setenv("SMTP_PASSWORD", "")
	t.Setenv("SMTP_FROM", "")
	t.Setenv("SMTP_FROM_NAME", "")
}

func TestLoad_SMTPDisabled_NoValidation(t *testing.T) {
	setEnv(t, allRequired)
	clearSMTP(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error with SMTP disabled: %v", err)
	}
	if cfg.SMTPHost != "" {
		t.Errorf("SMTPHost = %q, want empty", cfg.SMTPHost)
	}
	if cfg.SMTPPort != 587 {
		t.Errorf("SMTPPort = %d, want 587 (default)", cfg.SMTPPort)
	}
}

func TestLoad_SMTPFullyConfigured_Success(t *testing.T) {
	setEnv(t, allRequired)
	clearSMTP(t)
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "2525")
	t.Setenv("SMTP_USERNAME", "user")
	t.Setenv("SMTP_PASSWORD", "secret")
	t.Setenv("SMTP_FROM", "no-reply@example.com")
	t.Setenv("SMTP_FROM_NAME", "Example")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SMTPHost != "smtp.example.com" {
		t.Errorf("SMTPHost = %q", cfg.SMTPHost)
	}
	if cfg.SMTPPort != 2525 {
		t.Errorf("SMTPPort = %d", cfg.SMTPPort)
	}
	if cfg.SMTPUsername != "user" {
		t.Errorf("SMTPUsername = %q", cfg.SMTPUsername)
	}
	if cfg.SMTPFrom != "no-reply@example.com" {
		t.Errorf("SMTPFrom = %q", cfg.SMTPFrom)
	}
	if cfg.SMTPFromName != "Example" {
		t.Errorf("SMTPFromName = %q", cfg.SMTPFromName)
	}
}

func TestLoad_SMTPHostWithoutFrom_ReturnsError(t *testing.T) {
	setEnv(t, allRequired)
	clearSMTP(t)
	t.Setenv("SMTP_HOST", "smtp.example.com")
	// SMTP_FROM intentionally omitted.

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when SMTP_FROM is missing")
	}
	if !strings.Contains(err.Error(), "SMTP_FROM") {
		t.Errorf("error %q does not mention SMTP_FROM", err.Error())
	}
}

func TestLoad_SMTPMailpitStyle_NoCredentials_OK(t *testing.T) {
	// Mailpit accepts unauthenticated submissions, so username+password may both be empty.
	setEnv(t, allRequired)
	clearSMTP(t)
	t.Setenv("SMTP_HOST", "mailpit")
	t.Setenv("SMTP_PORT", "1025")
	t.Setenv("SMTP_FROM", "no-reply@charitychest.local")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error for unauthenticated SMTP: %v", err)
	}
	if cfg.SMTPHost != "mailpit" || cfg.SMTPPort != 1025 {
		t.Errorf("unexpected host/port: %q:%d", cfg.SMTPHost, cfg.SMTPPort)
	}
}

func TestLoad_SMTPUsernameWithoutPassword_ReturnsError(t *testing.T) {
	setEnv(t, allRequired)
	clearSMTP(t)
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_FROM", "no-reply@example.com")
	t.Setenv("SMTP_USERNAME", "user")
	// SMTP_PASSWORD intentionally omitted.

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when SMTP_USERNAME is set without SMTP_PASSWORD")
	}
	if !strings.Contains(err.Error(), "SMTP_PASSWORD") {
		t.Errorf("error %q does not mention SMTP_PASSWORD", err.Error())
	}
}

func TestLoad_SMTPPasswordWithoutUsername_ReturnsError(t *testing.T) {
	setEnv(t, allRequired)
	clearSMTP(t)
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_FROM", "no-reply@example.com")
	t.Setenv("SMTP_PASSWORD", "secret")
	// SMTP_USERNAME intentionally omitted.

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when SMTP_PASSWORD is set without SMTP_USERNAME")
	}
	if !strings.Contains(err.Error(), "SMTP_USERNAME") {
		t.Errorf("error %q does not mention SMTP_USERNAME", err.Error())
	}
}

func TestLoad_SMTPInvalidPort_ReturnsError(t *testing.T) {
	setEnv(t, allRequired)
	clearSMTP(t)
	t.Setenv("SMTP_PORT", "not-a-port")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid SMTP_PORT")
	}
	if !strings.Contains(err.Error(), "SMTP_PORT") {
		t.Errorf("error %q does not mention SMTP_PORT", err.Error())
	}
}

func TestLoad_SMTPPortOutOfRange_ReturnsError(t *testing.T) {
	setEnv(t, allRequired)
	clearSMTP(t)
	t.Setenv("SMTP_PORT", "70000")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for out-of-range SMTP_PORT")
	}
	if !strings.Contains(err.Error(), "SMTP_PORT") {
		t.Errorf("error %q does not mention SMTP_PORT", err.Error())
	}
}

func TestLoad_SMTPFromName_Default(t *testing.T) {
	setEnv(t, allRequired)
	clearSMTP(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SMTPFromName != "Charity Chest" {
		t.Errorf("SMTPFromName = %q, want default %q", cfg.SMTPFromName, "Charity Chest")
	}
}
