package main

import (
	"flag"
	"log"
	"os"

	"charity-chest/internal/config"
	"charity-chest/internal/model"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	email := flag.String("email", "", "root user email (or set SEED_ROOT_EMAIL)")
	password := flag.String("password", "", "root user password (or set SEED_ROOT_PASSWORD)")
	migrationsDir := flag.String("migrations", "migrations", "path to migrations directory")
	flag.Parse()

	_ = godotenv.Load()

	if *email == "" {
		*email = os.Getenv("SEED_ROOT_EMAIL")
	}
	if *password == "" {
		*password = os.Getenv("SEED_ROOT_PASSWORD")
	}
	if *email == "" || *password == "" {
		log.Fatal("email and password are required (flags -email/-password or env SEED_ROOT_EMAIL/SEED_ROOT_PASSWORD)")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	m, err := migrate.New("file://"+*migrationsDir, dbURL)
	if err != nil {
		log.Fatalf("migrate: create: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("migrate: up: %v", err)
	}

	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("database: %v", err)
	}

	var existing model.User
	rootExists := db.Where("role = ?", model.RoleRoot).First(&existing).Error == nil
	if rootExists {
		if config.AppEnv(os.Getenv("APP_ENV")) == config.AppEnvProduction {
			log.Fatal("seed-root is blocked in production when a root user already exists")
		}
		log.Printf("root user already exists (id=%d, email=%s) — nothing to do", existing.ID, existing.Email)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("bcrypt: %v", err)
	}
	hashStr := string(hash)
	role := model.RoleRoot

	user := model.User{
		Email:        *email,
		PasswordHash: &hashStr,
		Name:         "Root",
		Role:         &role,
	}
	if err := db.Create(&user).Error; err != nil {
		log.Fatalf("create root user: %v", err)
	}

	log.Printf("root user created (id=%d, email=%s)", user.ID, user.Email)
}
