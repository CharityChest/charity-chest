package main

import (
	"log"

	"charity-chest/internal/config"
	"charity-chest/internal/handler"
	"charity-chest/internal/routes"

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

	e := echo.New()
	e.HideBanner = true

	e.Use(echomw.Logger())
	e.Use(echomw.Recover())
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAuthorization},
	}))

	authHandler := handler.NewAuthHandler(db, cfg)

	routes.RegisterHealth(e)

	v1 := e.Group("/v1")
	routes.RegisterAuth(v1, authHandler)
	routes.RegisterAPI(v1, authHandler, cfg.JWTSecret)

	log.Printf("starting server on :%s", cfg.Port)
	log.Fatal(e.Start(":" + cfg.Port))
}
