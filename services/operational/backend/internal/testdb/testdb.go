// Package testdb provides a shared Postgres harness for the operational
// backend's tests. Mirrors services/admin/backend/internal/testdb almost
// verbatim. The migrations/ directory may be empty in v1 (operational has no
// owned entities yet) — migrate.Up() returns migrate.ErrNoChange which we
// ignore, so tests still work against a freshly-cloned blank template.
//
// This package is only meant to be imported from *_test.go files. Heavy deps
// (testcontainers, lib/pq) are never linked into the production binary
// because nothing under main imports it.
package testdb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // pg migration driver
	_ "github.com/golang-migrate/migrate/v4/source/file"       // file:// source
	_ "github.com/lib/pq"                                      // database/sql "postgres" driver
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	adminDBName    = "postgres"
	templateDBName = "cc_op_template"
)

var (
	startOnce sync.Once
	adminDSN  string
	startErr  error
)

// Open returns a fresh, migrated per-test database. The database is dropped
// on test cleanup. Safe to call from parallel tests.
func Open(t *testing.T) *gorm.DB {
	t.Helper()
	ensureStarted(t)

	name := newDBName(t)
	ctx := context.Background()

	if err := execAdmin(ctx, fmt.Sprintf(`CREATE DATABASE %q TEMPLATE %q`, name, templateDBName)); err != nil {
		t.Fatalf("testdb: create %q: %v", name, err)
	}

	db, err := gorm.Open(postgres.Open(rewriteDB(adminDSN, name)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		_ = execAdmin(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
		t.Fatalf("testdb: gorm open: %v", err)
	}

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		_ = execAdmin(context.Background(), fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
	})

	return db
}

func ensureStarted(t testing.TB) {
	startOnce.Do(func() { startErr = start() })
	if startErr != nil {
		t.Fatalf("testdb: start: %v", startErr)
	}
}

func start() error {
	ctx := context.Background()

	c, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase(adminDBName),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return fmt.Errorf("container: %w", err)
	}

	adminDSN, err = c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return fmt.Errorf("conn string: %w", err)
	}

	if err := execAdmin(ctx, fmt.Sprintf(`CREATE DATABASE %q`, templateDBName)); err != nil {
		return fmt.Errorf("create template: %w", err)
	}

	dir := migrationsDir()
	// In v1 there may not be any migration files at all; in that case skip
	// the migrate step entirely so the template DB stays empty.
	if hasMigrations(dir) {
		m, err := migrate.New("file://"+dir, rewriteDB(adminDSN, templateDBName))
		if err != nil {
			return fmt.Errorf("migrate new: %w", err)
		}
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			_, _ = m.Close()
			return fmt.Errorf("migrate up: %w", err)
		}
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			return fmt.Errorf("migrate close: src=%v db=%v", srcErr, dbErr)
		}
	}
	return nil
}

func execAdmin(ctx context.Context, query string) error {
	db, err := sql.Open("postgres", adminDSN)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	_, err = db.ExecContext(ctx, query)
	return err
}

// migrationsDir resolves the absolute path of the backend's migrations
// directory using the compiled-in location of this file.
func migrationsDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	// thisFile = .../services/operational/backend/internal/testdb/testdb.go
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")
}

// hasMigrations reports whether the directory contains any .sql files.
// golang-migrate fails with "first file not found" when pointed at an empty
// dir, so we short-circuit that case.
func hasMigrations(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			return true
		}
	}
	return false
}

// rewriteDB replaces the database segment of a libpq-style URL.
func rewriteDB(dsn, name string) string {
	slash := strings.LastIndex(dsn, "/")
	if slash == -1 {
		return dsn
	}
	rest := dsn[slash+1:]
	q := strings.Index(rest, "?")
	if q == -1 {
		return dsn[:slash+1] + name
	}
	return dsn[:slash+1] + name + rest[q:]
}

func newDBName(t *testing.T) string {
	t.Helper()
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("testdb: random name: %v", err)
	}
	return "cc_op_test_" + hex.EncodeToString(b[:])
}
