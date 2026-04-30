# Charity Chest

[![CI](https://github.com/CharityChest/charity-chest/actions/workflows/ci.yml/badge.svg)](https://github.com/CharityChest/charity-chest/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/badge/coverage-%E2%89%A580%25-brightgreen)](#)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](server/README.md)
[![Next.js](https://img.shields.io/badge/Next.js-15-000000?logo=nextdotjs&logoColor=white)](webapp/README.md)
[![Node](https://img.shields.io/badge/Node-20-339933?logo=nodedotjs&logoColor=white)](webapp/README.md)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Platform for managing charitable organisations. Handles user authentication, multi-factor authentication, and a two-tier role hierarchy — system administrators manage organisations; organisation owners and admins manage their own members.

---

## Features

- **Authentication** — email/password registration and login, Google OAuth
- **MFA** — TOTP-based two-factor authentication (RFC 6238), with QR code enrollment
- **Role hierarchy** — `root` → `system` → org-level `owner` / `admin` / `operational`
- **Organisation management** — CRUD for organisations; member invite and role assignment with hierarchy enforcement
- **Internationalisation** — English and Italian throughout (API error messages + full UI)
- **System configuration gate** — UI redirects to a setup page until a root user exists

---

## Repository layout

```
charity-chest/
├── server/    # Go HTTP API (Echo v4, GORM, PostgreSQL, JWT)
└── webapp/    # Next.js 15 frontend (TypeScript, Tailwind CSS, next-intl)
```

Both components are independent — they communicate over HTTP and can be deployed separately.

| Component | Docs |
|---|---|
| API server | [server/README.md](server/README.md) |
| Web application | [webapp/README.md](webapp/README.md) |

---

## Quick start

The fastest way to run everything locally is Docker Compose:

```bash
# 1. Server — copy secrets and add your Google OAuth credentials
cp server/.docker-dev/.env.example server/.docker-dev/.env

# 2. Webapp — set the API URL
cp webapp/.docker-dev/.env.example webapp/.docker-dev/.env

# 3. Start the API (Postgres + server, migrations run automatically)
docker compose -f server/.docker-dev/docker-compose.yml up --build

# 4. In another terminal, start the webapp
docker compose -f webapp/.docker-dev/docker-compose.yml up --build
```

| Service | URL |
|---|---|
| Web application | http://localhost:3000 |
| API server | http://localhost:8080 |

**Bootstrap the first root user** (run once after the server is up):

```bash
docker compose -f server/.docker-dev/docker-compose.yml run --rm \
  -e SEED_ROOT_EMAIL=admin@example.com \
  -e SEED_ROOT_PASSWORD=secret \
  server ./seed-root
```

After that, `GET /v1/system/status` returns `{"configured":true}` and the webapp grants normal access.

See the component READMEs for local (non-Docker) setup, environment variable reference, and deployment guides.

---

## Tech stack

| Layer | Technology |
|---|---|
| API framework | [Echo v4](https://echo.labstack.com/) |
| ORM | [GORM](https://gorm.io/) + PostgreSQL |
| Auth tokens | JWT (HS256, 24 h), TOTP via [pquerna/otp](https://github.com/pquerna/otp) |
| Frontend | [Next.js 15](https://nextjs.org/) (App Router) |
| Styling | [Tailwind CSS v3](https://tailwindcss.com/) |
| i18n | [next-intl](https://next-intl-docs.vercel.app/) |
| Server tests | Go standard `testing` package, SQLite in-memory |
| Webapp tests | [Vitest](https://vitest.dev/) + [React Testing Library](https://testing-library.com/) |

---

## CI

Every pull request must pass two independent checks before it can be merged:

| Check | What it runs | Coverage gate |
|---|---|---|
| **Server tests (Go)** | `go test -race ./internal/...` | ≥ 80% total |
| **Webapp tests (Node)** | `vitest run --coverage` | ≥ 80% lines / functions / branches / statements |

---

## License

[MIT](LICENSE)
