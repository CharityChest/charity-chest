BINARY  := server
DEBUG   := dist/debug/$(BINARY)
RELEASE := dist/release/$(BINARY)

# golang-migrate is already in go.mod — no separate install needed.
MIGRATE      := go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4

.PHONY: all build build-debug build-release test test-coverage lint fmt clean tidy migrate migrate-down migrate-version

# ── Default ───────────────────────────────────────────────────────────────────

all: build

build: build-debug build-release

# ── Debug build ───────────────────────────────────────────────────────────────
# Optimisations and inlining are disabled so the binary is debugger-friendly;
# the race detector is enabled to catch data races during development.

build-debug:
	@mkdir -p dist/debug
	go build \
	  -race \
	  -gcflags="all=-N -l" \
	  -o $(DEBUG) \
	  .
	@echo "debug binary → $(DEBUG)"

# ── Release build ─────────────────────────────────────────────────────────────
# Produces a fully static binary with debug info and symbol table stripped.

build-release:
	@mkdir -p dist/release
	CGO_ENABLED=0 GOOS=linux go build \
	  -ldflags="-s -w" \
	  -o $(RELEASE) \
	  .
	@echo "release binary → $(RELEASE)"

# ── Helpers ───────────────────────────────────────────────────────────────────

test:
	@docker info >/dev/null 2>&1 || (echo "ERROR: Docker daemon required for tests (testcontainers boots a Postgres container)"; exit 1)
	go test -race ./internal/...

# Report coverage across all production packages, excluding the entry point.
# `-coverpkg=./...` cross-instruments every package so integration coverage in
# routes/v1 counts against the file that owns the code.
test-coverage:
	@docker info >/dev/null 2>&1 || (echo "ERROR: Docker daemon required for tests (testcontainers boots a Postgres container)"; exit 1)
	go test -race -coverpkg=./... -coverprofile=coverage.out ./...
	@grep -v -E 'charity-chest/services/operational/backend/main\.go' coverage.out > coverage.business.out
	@echo "── full coverage ──"
	@go tool cover -func=coverage.out | tail -1
	@echo "── coverage excluding entry points ──"
	@go tool cover -func=coverage.business.out | tail -1

lint:
	$(GOLANGCI_LINT) run ./...

fmt:
	gofmt -w .
	$(GOLANGCI_LINT) run --fix ./...

tidy:
	go mod tidy

clean:
	rm -rf dist

# ── Migrations ────────────────────────────────────────────────────────────────
# DATABASE_URL must be set in the environment or .env. The migrations/
# directory may be empty in v1 — golang-migrate then reports no change.

migrate:
	@test -n "$(DATABASE_URL)" || (echo "ERROR: DATABASE_URL is required"; exit 1)
	$(MIGRATE) -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	@test -n "$(DATABASE_URL)" || (echo "ERROR: DATABASE_URL is required"; exit 1)
	@test -n "$(N)" || (echo "ERROR: N is required (e.g. make migrate-down N=1)"; exit 1)
	$(MIGRATE) -path migrations -database "$(DATABASE_URL)" down $(N)

migrate-version:
	@test -n "$(DATABASE_URL)" || (echo "ERROR: DATABASE_URL is required"; exit 1)
	$(MIGRATE) -path migrations -database "$(DATABASE_URL)" version
