package config_test

import (
	"strings"
	"testing"

	"charity-chest/services/operational/backend/internal/config"
)

// setEnvs sets the given env vars for the test and restores them on cleanup.
// Empty values unset the variable for the duration of the test.
func setEnvs(t *testing.T, kvs map[string]string) {
	t.Helper()
	for k, v := range kvs {
		t.Setenv(k, v)
	}
}

// validEnv returns the minimum set of vars Load() needs to succeed.
func validEnv() map[string]string {
	return map[string]string{
		"APP_ENV":          "local",
		"DATABASE_URL":     "postgres://x:y@z:5432/d?sslmode=disable",
		"JWT_SECRET":       "test-secret",
		"ADMIN_BASE_URL":   "http://admin:8080",
		"SERVICE_API_KEY":  "shared",
		"GOOGLE_AUDIENCE":  "client-id.apps.googleusercontent.com",
		"PORT":             "8081",
		"REQUEST_LOG_ENABLED": "false",
	}
}

func TestLoad_OK(t *testing.T) {
	setEnvs(t, validEnv())
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AppEnv != config.AppEnvLocal {
		t.Errorf("AppEnv = %q, want local", cfg.AppEnv)
	}
	if cfg.Port != "8081" {
		t.Errorf("Port = %q, want 8081", cfg.Port)
	}
	if cfg.RequestLogEnabled {
		t.Errorf("RequestLogEnabled = true, want false")
	}
}

func TestLoad_MissingRequired_ListsAll(t *testing.T) {
	// Set APP_ENV so we exercise the missing-vars accumulator instead of
	// the APP_ENV-specific error branch.
	t.Setenv("APP_ENV", "local")
	// Explicitly unset every other required var.
	for _, k := range []string{"DATABASE_URL", "JWT_SECRET", "ADMIN_BASE_URL", "SERVICE_API_KEY", "GOOGLE_AUDIENCE"} {
		t.Setenv(k, "")
	}
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	for _, want := range []string{"DATABASE_URL", "JWT_SECRET", "ADMIN_BASE_URL", "SERVICE_API_KEY", "GOOGLE_AUDIENCE"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}
}

func TestLoad_InvalidAppEnv(t *testing.T) {
	setEnvs(t, validEnv())
	t.Setenv("APP_ENV", "weird")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "APP_ENV") {
		t.Errorf("error %q does not mention APP_ENV", err)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	setEnvs(t, validEnv())
	t.Setenv("PORT", "70000")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "PORT") {
		t.Errorf("error %q does not mention PORT", err)
	}
}

func TestLoad_InvalidCacheTTL(t *testing.T) {
	setEnvs(t, validEnv())
	t.Setenv("CACHE_TTL", "not-a-duration")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "CACHE_TTL") {
		t.Errorf("error %q does not mention CACHE_TTL", err)
	}
}
