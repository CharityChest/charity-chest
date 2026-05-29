// Operational backend entry point.
//
// Bootstrap order mirrors services/admin/backend/main.go: config → migrations
// → GORM open → cache init → echo + middleware → routes → listen. Migrations
// are a no-op when migrations/ is empty (v1).
package main

import (
	"context"
	"errors"
	"log"
	"os"

	"charity-chest/services/operational/backend/internal/adminclient"
	"charity-chest/services/operational/backend/internal/cache"
	"charity-chest/services/operational/backend/internal/config"
	"charity-chest/services/operational/backend/internal/handler"
	"charity-chest/services/operational/backend/internal/middleware"
	routesv1 "charity-chest/services/operational/backend/internal/routes/v1"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"google.golang.org/api/idtoken"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Run SQL migrations from the ./migrations directory. The directory may
	// be empty (no operational entities in v1). golang-migrate's file source
	// surfaces "no files" two different ways depending on which entry point
	// you hit:
	//   • migrate.New errors when the directory itself doesn't exist;
	//   • m.Up returns os.ErrNotExist (wrapped as "first .: file does not
	//     exist") when the directory is present but holds no .sql files.
	// Both, plus migrate.ErrNoChange (nothing new to apply), are treated as
	// "no migrations to run" and not fatal.
	if m, err := migrate.New("file://migrations", cfg.DatabaseURL); err == nil {
		if upErr := m.Up(); upErr != nil &&
			!errors.Is(upErr, migrate.ErrNoChange) &&
			!errors.Is(upErr, os.ErrNotExist) {
			log.Fatalf("migrate: up: %v", upErr)
		}
		_, _ = m.Close()
	} else {
		log.Printf("migrate: skipped (%v)", err)
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	_ = db // reserved for future operational-owned entities

	var appCache *cache.Cache
	if cfg.CacheEnabled {
		appCache, err = cache.New(cfg.CacheURL, cfg.CacheTTL)
		if err != nil {
			log.Fatalf("cache: %v", err)
		}
		log.Printf("cache: enabled (TTL=%s)", cfg.CacheTTL)
	} else {
		appCache = cache.Disabled()
		log.Printf("cache: disabled")
	}
	_ = appCache // reserved for future cached endpoints

	adminAPI := adminclient.New(cfg.AdminBaseURL, cfg.ServiceAPIKey, cfg.AdminTimeout)

	googleValidator, err := newGoogleValidator(context.Background())
	if err != nil {
		log.Fatalf("google: validator: %v", err)
	}

	e := echo.New()
	e.HideBanner = true
	if cfg.RequestLogEnabled {
		e.Use(echomw.RequestLogger())
	}
	e.Use(echomw.Recover())
	e.Use(middleware.Locale())
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAuthorization, "X-Locale"},
	}))

	authH := handler.NewAuthHandler(cfg, adminAPI, googleValidator)
	meH := handler.NewMeHandler(adminAPI)

	routesv1.RegisterHealth(e)
	v1 := e.Group("/v1")
	routesv1.RegisterAuth(v1, authH)
	routesv1.RegisterAPI(v1, meH, cfg.JWTSecret)

	log.Printf("starting operational server on :%s (admin=%s)", cfg.Port, cfg.AdminBaseURL)
	log.Fatal(e.Start(":" + cfg.Port))
}

// realGoogleValidator wraps idtoken.NewValidator so it implements the
// handler.GoogleValidator interface without leaking the upstream library
// into the handler package.
type realGoogleValidator struct {
	v *idtoken.Validator
}

func newGoogleValidator(ctx context.Context) (*realGoogleValidator, error) {
	v, err := idtoken.NewValidator(ctx)
	if err != nil {
		return nil, err
	}
	return &realGoogleValidator{v: v}, nil
}

func (r *realGoogleValidator) Validate(ctx context.Context, idToken, audience string) (*handler.GooglePayload, error) {
	p, err := r.v.Validate(ctx, idToken, audience)
	if err != nil {
		return nil, err
	}
	email, _ := p.Claims["email"].(string)
	name, _ := p.Claims["name"].(string)
	return &handler.GooglePayload{Subject: p.Subject, Email: email, Name: name}, nil
}
