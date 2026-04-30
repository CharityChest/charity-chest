package config

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL        string
	JWTSecret          string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	FrontendURL        string
	Port               string
	CacheEnabled       bool
	CacheURL           string
	CacheTTL           time.Duration
}

// Load reads configuration from environment variables.
// It attempts to load a .env file first (silently ignored in production where it won't exist).
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  envOrDefault("GOOGLE_REDIRECT_URL", "http://localhost:8080/v1/auth/google/callback"),
		FrontendURL:        envOrDefault("FRONTEND_URL", "http://localhost:3000"),
		Port:               envOrDefault("PORT", "8080"),
		CacheEnabled:       os.Getenv("CACHE_ENABLED") == "true",
		CacheURL:           envOrDefault("CACHE_URL", "redis://localhost:6379"),
		CacheTTL:           parseDuration(os.Getenv("CACHE_TTL"), 5*time.Minute),
	}

	var missing []string
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

	if len(missing) > 0 {
		return nil, errors.New("missing required environment variables: " + strings.Join(missing, ", "))
	}

	return cfg, nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseDuration(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}