# Staging image — production-style build of the operational Node.js backend.
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

# ---- build stage (compile TypeScript) -----------------------------------
FROM node:24-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY tsconfig.json ./
COPY src ./src
RUN npm run build

# ---- deps stage (production node_modules) --------------------------------
FROM node:24-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --omit=dev

# ---- runtime stage ------------------------------------------------------
FROM node:24-alpine

# curl: used by the Docker HEALTHCHECK to probe /health.
# tzdata: consistent timezone handling.
# The node:alpine image already ships an unprivileged `node` user.
RUN apk --no-cache add ca-certificates tzdata curl

WORKDIR /app
ENV NODE_ENV=production

COPY --from=deps  --chown=root:root /app/node_modules ./node_modules
COPY --from=build --chown=root:root /app/dist         ./dist
COPY              --chown=root:root package.json       ./package.json
COPY              --chown=root:root migrations         ./migrations
COPY              --chown=root:root ./.docker-staging/entry-point.sh ./entry-point.sh
RUN chmod 0555 ./entry-point.sh

# Drop privileges — the app does not need root at runtime.
USER node

EXPOSE 8081

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -fsS http://localhost:8081/health || exit 1

ENTRYPOINT ["./entry-point.sh"]
