# Staging image — production-style build of the Go API server.
# The image is self-contained and designed to be deployed standalone
# (e.g. ECS, Fly.io, Kubernetes) connecting to managed Postgres and Valkey
# via environment variables supplied at runtime — there is no docker-compose
# for staging.
#
# Usage:
#   docker build \
#     -f server/.docker-staging/Dockerfile \
#     -t charity-chest-server:staging \
#     server/
#
# Required runtime env vars (see server/.env.example):
#   DATABASE_URL, JWT_SECRET, GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET,
#   APP_ENV=staging
# Optional: GOOGLE_REDIRECT_URL, FRONTEND_URL, PORT,
#           CACHE_ENABLED, CACHE_URL, CACHE_TTL,
#           STRIPE_SECRET_KEY, STRIPE_WEBHOOK_SECRET, STRIPE_PRO_PRICE_ID
#
# Auto-seed: when ROOT_USER and ROOT_PASSWORD are both set in the container
# environment, the entry-point seeds the first root user before starting the
# server. The seed is best-effort — a failure (e.g. user already exists) does
# not block startup. Leave them unset to skip seeding entirely.
#
# Migrations run automatically on startup.

# ---- build stage --------------------------------------------------------
FROM golang:alpine AS builder

WORKDIR /app

# Cache dependency downloads as a separate layer
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# -trimpath strips local filesystem paths from the binary for reproducibility.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/server . && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/seed-root ./cmd/seed-root

# ---- runtime stage ------------------------------------------------------
FROM alpine:3.23

# ca-certificates: HTTPS to Google OAuth and Stripe
# tzdata: consistent timezone handling
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -S app && adduser -S -G app app

WORKDIR /app

# Files are owned by root with read/execute for the unprivileged runtime user.
# A compromised process cannot tamper with the binary, migrations, or CA bundle.
COPY --from=builder --chown=root:root --chmod=0555 /bin/server      ./server
COPY --from=builder --chown=root:root --chmod=0555 /bin/seed-root   ./seed-root
# 0555 on the migrations tree: read for files, traverse for the directory itself.
COPY --from=builder --chown=root:root --chmod=0555 /app/migrations  ./migrations
COPY              --chown=root:root --chmod=0444 ./aws/postgres/global-bundle.pem ./
COPY              --chown=root:root --chmod=0555 ./.docker-staging/entry-point.sh ./entry-point.sh

# Drop privileges — the binary does not need root at runtime.
USER app

EXPOSE 8080

ENTRYPOINT ["./entry-point.sh"]
