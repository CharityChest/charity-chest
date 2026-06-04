# Unified development stack

This directory holds a single `docker-compose.yml` that brings up **every**
Charity Chest service at once — admin backend + admin frontend + operational
backend, with their supporting Postgres / Valkey / Mailpit containers.

It is a synthesis of the three per-service compose files:

- `services/admin/backend/.docker-dev/docker-compose.yml`
- `services/admin/frontend/.docker-dev/docker-compose.yml`
- `services/operational/backend/.docker-dev/docker-compose.yml`

The per-service compose files remain valid for working on one service in
isolation; this stack is the right choice when you need the full
admin+operational topology (e.g. exercising the `/v1/internal/*`
service-to-service API end to end).

---

## Quick start

```bash
cd .compose
cp .env.example .env       # fill in the required secrets
docker compose up --build
```

The stack validates required env vars at startup: missing values fail loudly
with `Set <VAR> in .compose/.env` rather than booting into a broken state.

---

## What's inside

Eight services on a single docker network named `charitychest`. The
operational backend reaches admin in-cluster via `http://admin-backend:8080`.

| Service          | Host port    | Notes                                                   |
|------------------|--------------|---------------------------------------------------------|
| `admin-postgres` | 5432         | admin database                                          |
| `admin-valkey`   | 6379         | admin cache                                             |
| `mailpit`        | 1025 / 8025  | SMTP capture + web UI (UI auth: `charity-chest:Secret`) |
| `admin-backend`  | 8080         | admin Go API                                            |
| `admin-frontend` | 3000         | Next.js dev server (hot reload via bind mount)          |
| `op-postgres`    | 5433         | operational database                                    |
| `op-valkey`      | 6380         | operational cache (wired but unused in v1)              |
| `op-backend`     | 8081         | operational Node.js API                                      |

---

## Environment variables

All variables live in a single `.compose/.env`. See `.env.example` for the
full list with comments.

### Required (no safe default — startup will refuse to proceed without these)

| Variable               | Used by                       | Notes                                                                                  |
|------------------------|-------------------------------|----------------------------------------------------------------------------------------|
| `GOOGLE_CLIENT_ID`     | `admin-backend`               | OAuth 2.0 Web client ID (admin webapp Google sign-in).                                 |
| `GOOGLE_CLIENT_SECRET` | `admin-backend`               | Matching client secret.                                                                |
| `ROOT_USER`            | `admin-backend`               | Email of the root user seeded on first boot.                                           |
| `ROOT_PASSWORD`        | `admin-backend`               | Password for that root user.                                                           |
| `SERVICE_API_KEY`      | `admin-backend` + `op-backend` | Shared service-to-service secret. Generate with `openssl rand -hex 32`. Declared once and passed to both containers — no duplication. |
| `GOOGLE_AUDIENCE`      | `op-backend`                  | Comma-separated Google OAuth client ID(s) (iOS, Android, Web) — the mobile app's ID token `aud` must match one. |

### Optional (commented out in `.env.example`)

| Variable                | Default                  | Effect when blank                                       |
|-------------------------|--------------------------|---------------------------------------------------------|
| `NEXT_PUBLIC_API_URL`   | `http://localhost:8080`  | Browser-facing admin API URL.                           |
| `STRIPE_SECRET_KEY`     | unset                    | admin billing endpoints return 503                      |
| `STRIPE_WEBHOOK_SECRET` | unset                    | required by `config.Load()` once `STRIPE_SECRET_KEY` is set |
| `STRIPE_PRO_PRICE_ID`   | unset                    | same                                                    |

---

## Differences from the per-service compose files

The synthesis isn't a literal copy-paste — a few things had to change so the
three stacks can coexist:

- **Service renames.** Both `services/admin/backend/...` and
  `services/operational/backend/...` named their server container `server`.
  Here they're `admin-backend` and `op-backend` so DNS resolves
  unambiguously inside the shared network.
- **Single network.** The original operational compose joined admin's
  `charitychest_admin` network as `external: true`. With everything in one
  compose, the cross-stack network dance is unnecessary — both backends sit
  on the default `charitychest` network.
- **Single `SERVICE_API_KEY`.** Previously you had to set the same value in
  two `.env` files (admin's and operational's). Now it's declared once in
  `.compose/.env` and docker-compose passes it to both containers.
- **Frontend env inlined.** The frontend compose used `env_file: - .env`;
  here `NEXT_PUBLIC_API_URL` is set inline so a single root `.env` drives
  the whole stack.
- **Frontend `depends_on: admin-backend`.** Added so `docker compose up`
  starts services in a sensible order.

Everything else (image versions, ports, volume names, healthchecks, the
Mailpit + admin SMTP wiring) carries over verbatim.

---

## Caveats

- **Don't move this file.** Build contexts use relative paths
  (`../services/admin/backend`, `../services/admin/frontend`,
  `../services/operational/backend`). Move the compose file and the contexts
  break.
- **The admin backend Dockerfile copies `./aws/postgres/global-bundle.pem`**
  from its build context. That file already exists at
  `services/admin/backend/aws/postgres/global-bundle.pem`, so the build
  still works from `.compose/`.
- **Per-service compose files are still there.** They remain valid for
  working on one service in isolation. If you want a single source of truth,
  retire them — but until then, picking the wrong file means a partial
  stack.
- **Host port collisions.** This stack exposes 3000, 8080, 8081, 8025, 1025,
  5432, 5433, 6379, 6380. If you also have the per-service composes
  running, every one of those will collide.

---

## Common operations

```bash
# Full stack, rebuilt images, foreground
docker compose -f .compose/docker-compose.yml up --build

# Background
docker compose -f .compose/docker-compose.yml up -d --build

# Tail logs from one service
docker compose -f .compose/docker-compose.yml logs -f admin-backend

# Tear down (keeps volumes, so DB + node_modules survive)
docker compose -f .compose/docker-compose.yml down

# Tear down AND wipe data (DBs reset, Next.js cache cleared, etc.)
docker compose -f .compose/docker-compose.yml down -v
```
