package main

import (
	"log"

	"charity-chest/internal/cache"
	"charity-chest/internal/config"
	"charity-chest/internal/handler"
	"charity-chest/internal/middleware"
	routesv1 "charity-chest/internal/routes/v1"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Run SQL migrations from the ./migrations directory
	m, err := migrate.New("file://migrations", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("migrate: create: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("migrate: up: %v", err)
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("database: %v", err)
	}

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

	e := echo.New()
	e.HideBanner = true

	e.Use(echomw.RequestLogger())
	e.Use(echomw.Recover())
	e.Use(middleware.Locale())
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAuthorization, "X-Locale"},
	}))

	authHandler := handler.NewAuthHandler(db, cfg, appCache)

	routesv1.RegisterHealth(e)

	v1 := e.Group("/v1")
	routesv1.RegisterAuth(v1, authHandler)
	routesv1.RegisterAPI(v1, authHandler, cfg.JWTSecret)
	routesv1.RegisterSystem(v1, db, appCache, cfg.JWTSecret)
	routesv1.RegisterOrgs(v1, db, appCache, cfg.JWTSecret)
	routesv1.RegisterProfile(v1, db, cfg, appCache, cfg.JWTSecret)
	routesv1.RegisterAdmin(v1, db, appCache, cfg.JWTSecret)
	routesv1.RegisterBilling(e, v1, db, appCache, cfg, cfg.JWTSecret)

	log.Printf("starting server on :%s", cfg.Port)
	log.Fatal(e.Start(":" + cfg.Port))
}
