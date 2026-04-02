# CLAUDE.md — Charity Chest

This file is read by Claude Code at the start of every session. It describes the project layout, conventions, and rules to follow when working in this repository.

---

## Repository layout

```
charity-chest/
└── server/                         # Go HTTP API (the only service so far)
    ├── main.go                     # Entry point: config → migrations → routes → listen
    ├── go.mod / go.sum             # Module: charity-chest, Go 1.26
    ├── Makefile                    # Build, test, and utility targets
    ├── .env.example                # Template for local secrets (never commit .env)
    ├── .gitignore
    ├── internal/
    │   ├── config/config.go        # Loads env vars via godotenv; fails fast on missing required vars
    │   ├── handler/auth.go         # Register, Login, GoogleLogin, GoogleCallback, Me
    │   ├── middleware/jwt.go       # Bearer token validation; injects user_id + email into context
    │   └── model/user.go           # GORM User model (supports password + Google OAuth)
    ├── migrations/                 # Raw SQL migrations (golang-migrate, file source)
    │   ├── 000001_create_users_table.up.sql
    │   └── 000001_create_users_table.down.sql
    └── .docker-dev/                # Docker Compose demo environment
        ├── Dockerfile              # Two-stage build (golang:alpine → alpine)
        ├── docker-compose.yml      # Postgres + server; server waits for DB health check
        └── .env.example            # Template for Google OAuth secrets used by compose
```

---

## Server tech stack

| Concern | Library |
|---|---|
| HTTP framework | `github.com/labstack/echo/v4` |
| ORM | `gorm.io/gorm` + `gorm.io/driver/postgres` |
| Migrations | `github.com/golang-migrate/migrate/v4` (SQL files in `migrations/`) |
| Auth tokens | `github.com/golang-jwt/jwt/v5` (HS256, 24 h expiry) |
| Password hashing | `golang.org/x/crypto/bcrypt` (DefaultCost) |
| Google OAuth | `golang.org/x/oauth2` + `golang.org/x/oauth2/google` |
| Config / secrets | `github.com/joho/godotenv` (dev only) + `os.Getenv` |
| Test DB | `github.com/glebarez/sqlite` (pure Go, in-memory) |

---

## API versioning

All application routes are prefixed with `/v1/`. The health probe is intentionally unversioned — it is an infrastructure endpoint, not part of the API contract.

When a breaking change is needed, introduce a `/v2/` group in `main.go` alongside `/v1/` and keep both alive until clients have migrated.

## API surface

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/health` | — | Liveness probe (unversioned) |
| POST | `/v1/auth/register` | — | Create account (email + password) → JWT |
| POST | `/v1/auth/login` | — | Password login → JWT |
| GET | `/v1/auth/google` | — | Redirect to Google consent screen |
| GET | `/v1/auth/google/callback` | — | Exchange OAuth code → JWT |
| GET | `/v1/api/me` | Bearer JWT | Return current user |

Protected routes live under `/v1/api/` and require a valid `Authorization: Bearer <token>` header. The JWT middleware (`internal/middleware/jwt.go`) validates the token and injects `user_id` (uint) and `email` (string) into the Echo context.

---

## Secret management rules

- Secrets live **only in environment variables** — never hardcoded, never committed.
- `server/.env` is git-ignored. Copy `server/.env.example` to create it locally.
- `server/.docker-dev/.env` is also git-ignored. Copy `server/.docker-dev/.env.example`.
- `config.Load()` (`internal/config/config.go`) calls `godotenv.Load()` silently (ignored in production) then validates all required vars, returning an error that names every missing variable.
- Required vars: `DATABASE_URL`, `JWT_SECRET`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`.

---

## Database migrations

- Migrations are plain SQL files in `server/migrations/`, named `NNNNNN_<description>.{up,down}.sql`.
- They run automatically when the server starts (`migrate.Up()` in `main.go`).
- `ErrNoChange` is silently ignored; any other error is fatal.
- Never modify an already-applied migration file — add a new one instead.

---

## Development workflow

```bash
# Run locally (requires .env and a running Postgres)
cd server
go run .

# Run with Docker (no local Go/Postgres needed)
docker compose -f server/.docker-dev/docker-compose.yml up --build

# Build
make build-debug    # dist/debug/server   — race detector, no optimisations
make build-release  # dist/release/server — static, stripped

# Test (no external services needed — uses SQLite in-memory)
make test           # go test -race ./internal/...

# Tidy dependencies
make tidy
```

---

## Code conventions

- **Package layout**: all non-main code lives under `internal/`. No `pkg/` directory.
- **Error handling**: handlers return `echo.NewHTTPError(statusCode, message)`. Errors are never swallowed silently.
- **No user enumeration**: login returns a generic 401 for both "user not found" and "wrong password".
- **Sensitive fields**: `PasswordHash` and `GoogleID` are tagged `json:"-"` — they must never appear in API responses.
- **Nullable columns**: `PasswordHash` and `GoogleID` are `*string`; nil means that auth method is not configured for that user.
- **Tests**: one `_test.go` file per source file, in `package foo_test` (black-box). Each test gets a fresh in-memory SQLite DB via `newTestDB(t)`. No external services, no global state.
- **No testify**: tests use only the standard `testing` package.
- **Migrations**: always add a matching `.down.sql` for every `.up.sql`.

---

## Adding a new API endpoint

1. Add the handler method to the appropriate file in `internal/handler/` (or create a new file for a new domain).
2. Register the route under the `v1` group in `main.go`. Public routes go under `v1.Group("/auth")`; protected routes go under `v1.Group("/api")`.
3. If the endpoint needs a new table or column, create `migrations/NNNNNN_<description>.{up,down}.sql`.
4. Add tests in the corresponding `_test.go` file. Wire routes with the `/v1/` prefix in the `newServer` test helper.