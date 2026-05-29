# Charity Chest

[![CI](https://github.com/CharityChest/charity-chest/actions/workflows/ci.yml/badge.svg)](https://github.com/CharityChest/charity-chest/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/badge/coverage-%E2%89%A580%25-brightgreen)](#)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](services/admin/backend/README.md)
[![Next.js](https://img.shields.io/badge/Next.js-15-000000?logo=nextdotjs&logoColor=white)](services/admin/frontend/README.md)
[![Node](https://img.shields.io/badge/Node-20-339933?logo=nodedotjs&logoColor=white)](services/admin/frontend/README.md)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Platform for managing charitable organisations. Handles user authentication, multi-factor authentication, and a two-tier role hierarchy — system administrators manage organisations; organisation owners and admins manage their own members.

---

## Features

- **Authentication** — email/password registration and login, Google OAuth
- **MFA** — TOTP-based two-factor authentication (RFC 6238), with QR code enrollment
- **Password recovery** — self-service "forgot password" flow with single-use, time-limited email tokens; Mailpit is wired into the dev compose so recovery emails never leave the developer's machine
- **Role hierarchy** — `root` → `system` → org-level `owner` / `admin` / `operational`
- **Organisation management** — CRUD for organisations; member invite and role assignment with hierarchy enforcement
- **Internationalisation** — English and Italian throughout (API error messages + email content + full UI)
- **System configuration gate** — UI redirects to a setup page until a root user exists

---

## Repository layout

```
charity-chest/
├── .compose/                  # unified dev stack — brings up every service at once
└── services/
    ├── admin/                 # the "admin" microservice (system + organisation management)
    │   ├── backend/   # Go HTTP API (Echo v4, GORM, PostgreSQL, JWT)
    │   └── frontend/  # Next.js 15 webapp (TypeScript, Tailwind CSS, next-intl)
    └── operational/           # the "operational" microservice (end-user mobile surface)
        ├── backend/   # Go HTTP gateway; stateless re: user data, delegates to admin via /v1/internal/*
        └── app/       # Expo + React Native + TypeScript mobile app (iOS + Android)
```

A microservices monorepo: each service lives under `services/<name>/` with an independent `backend/` and `frontend/` (or `app/` for mobile) that communicate over HTTP and deploy separately. There are two services today, **admin** and **operational** — admin owns identity and organisation state; operational is a thin authenticated gateway for the mobile app that delegates every user read/write to admin via a service-to-service API.

| Component | Docs |
|---|---|
| Admin API | [services/admin/backend/README.md](services/admin/backend/README.md) |
| Admin web application | [services/admin/frontend/README.md](services/admin/frontend/README.md) |
| Operational API | [services/operational/backend/README.md](services/operational/backend/README.md) |
| Operational mobile app | [services/operational/app/README.md](services/operational/app/README.md) |
| Unified dev stack | [.compose/README.md](.compose/README.md) |

---

## Quick start

The fastest way to run everything locally is the **unified Docker Compose stack** in [`.compose/`](.compose/README.md). It brings up admin (Postgres + Valkey + Mailpit + Go API + Next.js webapp) and operational (its own Postgres + Valkey + Go API) on a single docker network, so the operational backend can reach admin in-cluster via `http://admin-backend:8080` without any external-network wiring.

```bash
# 1. Copy the env template and fill in the secrets that have no safe default
#    (Google OAuth, root user, SERVICE_API_KEY, GOOGLE_AUDIENCE — see comments in the file)
cp .compose/.env.example .compose/.env

# 2. Bring everything up
docker compose -f .compose/docker-compose.yml up --build
```

| Service | URL |
|---|---|
| Admin web application | http://localhost:3000 |
| Admin API | http://localhost:8080 |
| Operational API | http://localhost:8081 |
| Mailpit inbox (recovery emails) | http://localhost:8025 |

The root user is seeded automatically on first boot from `ROOT_USER` / `ROOT_PASSWORD` in `.compose/.env`. After that, `GET /v1/system/status` returns `{"configured":true}` and the webapp grants normal access.

Full reference for the unified stack — every service, every env var, common `up` / `logs` / `down` recipes, and how it differs from the per-service composes — is in **[.compose/README.md](.compose/README.md)**.

### Running a single service in isolation

If you only need one service (e.g. iterating on the admin backend without the operational stack in the way), the original per-service compose files are still wired up and remain the right choice:

```bash
# Admin backend + its Postgres + Valkey + Mailpit
docker compose -f services/admin/backend/.docker-dev/docker-compose.yml up --build

# Admin frontend (expects the admin backend to be reachable at NEXT_PUBLIC_API_URL)
docker compose -f services/admin/frontend/.docker-dev/docker-compose.yml up --build

# Operational backend (expects the admin backend on the shared `charitychest_admin` network)
docker compose -f services/operational/backend/.docker-dev/docker-compose.yml up --build
```

Each of those has its own `.docker-dev/.env.example`. Don't mix the two modes — the unified stack and the per-service composes expose the same host ports and will collide.

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
| Server tests | Go standard `testing` package, real Postgres in `postgres:16-alpine` via [testcontainers-go](https://golang.testcontainers.org/) (Docker required) |
| Webapp tests | [Vitest](https://vitest.dev/) + [React Testing Library](https://testing-library.com/) |

---

## CI

Every pull request must pass two independent checks before it can be merged:

| Check | What it runs | Coverage gate |
|---|---|---|
| **Server tests (Go)** | `go test -race -coverprofile=coverage.out ./internal/...` (locally: `make test-coverage`) | ≥ 80% total across `./internal/...` (main.go and `cmd/*` are skipped because they're thin entry points; templ-generated `*_templ.go` is included in the average but exercised through its package tests) |
| **Webapp tests (Node)** | `vitest run --coverage` | ≥ 80% lines / functions / branches / statements |

---

## Deploy

`.github/workflows/deploy.yml` runs on every push to `main` (i.e. every merged PR). It is a two-phase pipeline that produces a single multi-architecture image (`linux/amd64` + `linux/arm64`) per component and ships it to ECS:

1. **Build** — four parallel jobs (`admin-backend`/`admin-frontend` × `amd64`/`arm64`). Each job runs on the runner whose architecture matches the build target (`ubuntu-24.04` for amd64, `ubuntu-24.04-arm` for arm64 — both pinned to 24.04 rather than `-latest` so the runner image is stable across GitHub's `-latest` label moves), so builds are native — no QEMU, no `next build` slowdown on arm64. Each job pushes the image to ECR *by digest only* (no tag attached) and exports the digest as a workflow artifact.
2. **Manifest** — two jobs (one per component) download the two per-arch digests, stitch them into a single multi-arch manifest under `:<commit-sha>` and `:latest` via `docker buildx imagetools create`, then force a new ECS deployment so the service pulls the fresh image.

Configure the following in **Settings → Secrets and variables → Actions**:

| Kind | Name | Purpose |
|---|---|---|
| Secret | `AWS_ACCESS_KEY_ID` | IAM access key for an account with `ecr:*` (push) and `ecs:UpdateService` permissions |
| Secret | `AWS_SECRET_ACCESS_KEY` | Matching secret access key |
| Variable | `AWS_REGION_ADMIN` | e.g. `eu-west-1` |
| Variable | `ECR_REPOSITORY_ADMIN_BACKEND` | ECR repository name for the admin backend image |
| Variable | `ECR_REPOSITORY_ADMIN_FRONTEND` | ECR repository name for the admin frontend image |
| Variable | `ECS_CLUSTER_ADMIN` | ECS cluster name hosting the admin service |
| Variable | `ECS_SERVICE_ADMIN_BACKEND` | ECS service name for the admin backend |
| Variable | `ECS_SERVICE_ADMIN_FRONTEND` | ECS service name for the admin frontend |
| Variable | `NEXT_PUBLIC_API_URL_ADMIN` | Public API URL inlined into the admin frontend bundle at build time (e.g. `https://api.staging.example.com`); passed as the `NEXT_PUBLIC_API_URL` build arg |

The ECR repositories and ECS services must exist before the first run — the workflow does not create them.

### ECS task-definition architecture

ECS task definitions pin the CPU architecture via `runtimePlatform.cpuArchitecture` (`X86_64` or `ARM64`). Fargate selects the matching variant from the multi-arch manifest automatically — switching a service from amd64 to Graviton (arm64) is a task-definition change, not a workflow change. Make sure the Fargate capacity provider supports the target architecture before flipping it.

### Runner cost note

`ubuntu-24.04-arm` is free on public repos and billed at the standard arm64 rate on private repos (currently cheaper per minute than amd64). If the doubled job count becomes a concern on a private repo, the cheaper alternative is to drop one architecture from the matrix — emulating the missing arch with QEMU on a single runner is technically possible but the webapp's `next build` under emulation typically costs more wall-clock (and therefore more minutes) than a second native job.

---

## License

[MIT](LICENSE)
