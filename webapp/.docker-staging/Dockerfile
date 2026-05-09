# Staging image — production build of the Next.js webapp.
# The image is self-contained: no source mount at runtime.
#
# NEXT_PUBLIC_API_URL is inlined into the browser bundle by `next build`,
# so it MUST be supplied as a build arg. Setting it only at `docker run`
# time has no effect on the client bundle.
#
# Usage:
#   docker build \
#     -f webapp/.docker-staging/Dockerfile \
#     --build-arg NEXT_PUBLIC_API_URL=https://api.staging.example.com \
#     -t charity-chest-webapp:staging \
#     webapp/

# --- deps stage ----------------------------------------------------------
FROM node:24-alpine3.23 AS deps
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci

# --- builder stage -------------------------------------------------------
FROM node:24-alpine3.23 AS builder
WORKDIR /app

ARG NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ENV NEXT_TELEMETRY_DISABLED=1
ENV NODE_ENV=production

COPY --from=deps /app/node_modules ./node_modules
COPY . .

# Build, then drop the build cache so it is not shipped in the image and
# the runner cannot rely on it (the read-only runner FS would block writes anyway).
RUN npm run build && rm -rf .next/cache

# Drop devDependencies so the runner copies a lean tree.
RUN npm prune --omit=dev

# --- runner stage --------------------------------------------------------
FROM node:24-alpine3.23 AS runner
WORKDIR /app

# curl: used by HEALTHCHECK to probe the local Next.js server.
RUN apk --no-cache add curl

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000

# Files are owned by root with read/execute for the unprivileged runtime user.
# A compromised process cannot tamper with the bundle.
COPY --from=builder --chown=root:root --chmod=0555 /app/node_modules     ./node_modules
COPY --from=builder --chown=root:root --chmod=0555 /app/.next            ./.next
COPY --from=builder --chown=root:root --chmod=0555 /app/messages         ./messages
COPY --from=builder --chown=root:root --chmod=0555 /app/src              ./src
COPY --from=builder --chown=root:root --chmod=0444 /app/next.config.ts   ./next.config.ts
COPY --from=builder --chown=root:root --chmod=0444 /app/package.json     ./package.json
COPY --from=builder --chown=root:root --chmod=0444 /app/package-lock.json ./package-lock.json

# Next.js writes its runtime cache (fetch cache, image optimization) under
# .next/cache. With .next/ read-only, route those writes to /tmp via a
# symlink. /tmp/next-cache is owned by `node` with 0700, and lives on the
# container's ephemeral filesystem (tmpfs-friendly) so nothing survives
# restart. The symlink is created as root because .next/ is 0555 (root
# bypasses DAC inside the container).
RUN mkdir -p /tmp/next-cache && \
    chown node:node /tmp/next-cache && \
    chmod 0700 /tmp/next-cache && \
    ln -sfn /tmp/next-cache /app/.next/cache

# Drop privileges — Next.js does not need root at runtime.
USER node

EXPOSE 3000

# Probe the EN home page directly so orchestrators (ECS, Kubernetes, Fly.io)
# can mark the container unhealthy when Next.js stops responding. Hitting
# /en skips the next-intl redirect from "/" and exercises a real rendered
# page; `curl -f` exits non-zero on any 4xx/5xx so the probe goes red.
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
  CMD curl -fsS -o /dev/null http://127.0.0.1:3000/en || exit 1

CMD ["npm", "run", "start"]
