BINARY  := server
DEBUG   := dist/debug/$(BINARY)
RELEASE := dist/release/$(BINARY)

.PHONY: all build build-debug build-release test clean tidy seed-root

# ── Default ───────────────────────────────────────────────────────────────────

all: build

build: build-debug build-release

# ── Debug build ───────────────────────────────────────────────────────────────
# Optimisations and inlining are disabled so the binary is debugger-friendly.
# The race detector is enabled to catch data races during development.

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
	go test -race ./internal/...

tidy:
	go mod tidy

clean:
	rm -rf dist

# ── Seed ──────────────────────────────────────────────────────────────────────
# Creates a root user in the database.
# Usage: make seed-root EMAIL=admin@example.com PASSWORD=secret

seed-root:
	@test -n "$(EMAIL)"    || (echo "ERROR: EMAIL is required";    exit 1)
	@test -n "$(PASSWORD)" || (echo "ERROR: PASSWORD is required"; exit 1)
	SEED_ROOT_EMAIL=$(EMAIL) SEED_ROOT_PASSWORD=$(PASSWORD) \
	  go run ./cmd/seed-root/main.go -migrations migrations