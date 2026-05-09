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

RUN npm run build

# Drop devDependencies so the runner copies a lean tree.
RUN npm prune --omit=dev

# --- runner stage --------------------------------------------------------
FROM node:24-alpine3.23 AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000

# Run as the unprivileged `node` user shipped in the base image.
USER node

COPY --from=builder --chown=node:node /app/node_modules ./node_modules
COPY --from=builder --chown=node:node /app/.next ./.next
COPY --from=builder --chown=node:node /app/messages ./messages
COPY --from=builder --chown=node:node /app/src ./src
COPY --from=builder --chown=node:node /app/next.config.ts ./next.config.ts
COPY --from=builder --chown=node:node /app/package.json ./package.json
COPY --from=builder --chown=node:node /app/package-lock.json ./package-lock.json

EXPOSE 3000

CMD ["npm", "run", "start"]
