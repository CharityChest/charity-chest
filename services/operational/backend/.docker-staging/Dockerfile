# Staging image — production-style build of the operational Go backend.
# Self-contained, designed to be deployed standalone (ECS, Fly.io, Kubernetes)
# connecting to managed Postgres and Valkey via env vars supplied at runtime.
# No docker-compose for staging.
#
# Usage:
#   docker build \
#     -f services/operational/backend/.docker-staging/Dockerfile \
#     -t charity-chest-operational-server:staging \
#     services/operational/backend/
#
# Required runtime env vars (see services/operational/backend/.env.example):
#   APP_ENV, DATABASE_URL, JWT_SECRET, ADMIN_BASE_URL, SERVICE_API_KEY,
#   GOOGLE_AUDIENCE
# Optional: PORT, REQUEST_LOG_ENABLED, ADMIN_TIMEOUT, CACHE_*

# ---- build stage --------------------------------------------------------
FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/server .

# ---- runtime stage ------------------------------------------------------
FROM alpine:3.23

# ca-certificates: HTTPS to Google's tokeninfo and admin over TLS.
# tzdata: consistent timezone handling.
# curl: used by the Docker HEALTHCHECK to probe /health.
# adduser -H: no home directory the app user could write to.
RUN apk --no-cache add ca-certificates tzdata curl && \
    addgroup -S app && adduser -S -H -G app app

WORKDIR /app

# Lock the working directory so the unprivileged user cannot create or replace
# files alongside the binary at runtime.
RUN chmod 0555 /app

COPY --from=builder --chown=root:root --chmod=0555 /bin/server      ./server
COPY --from=builder --chown=root:root --chmod=0555 /app/migrations  ./migrations
COPY              --chown=root:root --chmod=0555 ./.docker-staging/entry-point.sh ./entry-point.sh

# Drop privileges — the binary does not need root at runtime.
USER app

EXPOSE 8081

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -fsS http://localhost:8081/health || exit 1

ENTRYPOINT ["./entry-point.sh"]
