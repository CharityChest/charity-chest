# Build context is the services/operational/backend/ directory
# (set by docker-compose context: ..).
#
# Multi-stage Node.js build: compile TypeScript → dist in the build stage,
# install production-only dependencies in the deps stage, then assemble a slim
# runtime image carrying only node_modules + dist + migrations.

# ---- build stage (compile TypeScript) ----
FROM node:24-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY tsconfig.json ./
COPY src ./src
RUN npm run build

# ---- deps stage (production node_modules) ----
FROM node:24-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --omit=dev

# ---- runtime stage ----
FROM node:24-alpine
WORKDIR /app

ENV NODE_ENV=production

COPY --from=deps  /app/node_modules ./node_modules
COPY --from=build /app/dist         ./dist
COPY package.json ./
COPY migrations ./migrations
COPY ./.docker-dev/entry-point.sh ./entry-point.sh
RUN chmod +x ./entry-point.sh

EXPOSE 8081

ENTRYPOINT ["./entry-point.sh"]
