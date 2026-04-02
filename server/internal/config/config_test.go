package config_test

import (
	"strings"
	"testing"

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