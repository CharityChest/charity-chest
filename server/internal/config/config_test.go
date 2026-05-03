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

func TestLoad_AppEnv_IsLoaded(t *testing.T) {
	setEnv(t, allRequired)
	t.Setenv("APP_ENV", "production")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AppEnv != "production" {
		t.Errorf("AppEnv = %q, want production", cfg.AppEnv)
	}
}

func TestLoad_AppEnv_DefaultIsEmpty(t *testing.T) {
	setEnv(t, allRequired)
	t.Setenv("APP_ENV", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AppEnv != "" {
		t.Errorf("AppEnv = %q, want empty string", cfg.AppEnv)
	}
}