// Package config loads runtime configuration from environment variables.
// Mirrors the shape and error-reporting style of services/admin/backend/internal/config
// so behaviour stays consistent across services.
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
//
// The operational backend never stores user data — every identity read/write
// goes through the admin backend's /v1/internal/* API. It does keep its own
// Postgres + cache scaffolding for future operational-owned entities.
type Config struct {
	DatabaseURL       string
	JWTSecret         string
	AdminBaseURL      string
	ServiceAPIKey     string
	GoogleAudience    string
	AdminTimeout      time.Duration
	Port              string
	AppEnv            AppEnv
	CacheEnabled      bool
	CacheURL          string
	CacheTTL          time.Duration
	RequestLogEnabled bool
}

// Load reads configuration from environment variables, populating defaults
// and returning a single error listing every missing required variable.
// Silently loads a .env file if one exists (ignored in production).
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		JWTSecret:         os.Getenv("JWT_SECRET"),
		AdminBaseURL:      os.Getenv("ADMIN_BASE_URL"),
		ServiceAPIKey:     os.Getenv("SERVICE_API_KEY"),
		GoogleAudience:    os.Getenv("GOOGLE_AUDIENCE"),
		Port:              envOrDefault("PORT", "8081"),
		CacheEnabled:      os.Getenv("CACHE_ENABLED") == "true",
		CacheURL:          envOrDefault("CACHE_URL", "redis://localhost:6379"),
		RequestLogEnabled: os.Getenv("REQUEST_LOG_ENABLED") != "false",
	}

	cacheTTL, err := parseDuration(os.Getenv("CACHE_TTL"), 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("invalid CACHE_TTL: %w", err)
	}
	cfg.CacheTTL = cacheTTL

	adminTimeout, err := parseDuration(os.Getenv("ADMIN_TIMEOUT"), 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("invalid ADMIN_TIMEOUT: %w", err)
	}
	cfg.AdminTimeout = adminTimeout

	var missing []string

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
	if cfg.AdminBaseURL == "" {
		missing = append(missing, "ADMIN_BASE_URL")
	}
	if cfg.ServiceAPIKey == "" {
		missing = append(missing, "SERVICE_API_KEY")
	}
	if cfg.GoogleAudience == "" {
		missing = append(missing, "GOOGLE_AUDIENCE")
	}

	if len(missing) > 0 {
		return nil, errors.New("missing required environment variables: " + strings.Join(missing, ", "))
	}

	// Validate PORT range — invalid here means the listener will fail later
	// with a less actionable error.
	if p, err := strconv.Atoi(cfg.Port); err != nil || p < 1 || p > 65535 {
		return nil, fmt.Errorf("invalid PORT %q", cfg.Port)
	}

	return cfg, nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseDuration(s string, def time.Duration) (time.Duration, error) {
	if s == "" {
		return def, nil
	}
	return time.ParseDuration(s)
}
