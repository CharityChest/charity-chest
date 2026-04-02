# Development-only image — hot reload via `next dev`.
# Source code is mounted as a volume at runtime; this image just installs dependencies.
#
# Usage:
#   docker compose -f webapp/.docker-dev/docker-compose.yml up --build

FROM node:20-alpine

WORKDIR /app

# Install dependencies into the image so they end up in a named volume
# and are not overwritten by the source mount (see docker-compose.yml).
COPY package.json package-lock.json* ./
RUN npm ci

# Source is mounted at runtime — nothing else to COPY here.
EXPOSE 3000

# Disable Next.js telemetry in all environments.
ENV NEXT_TELEMETRY_DISABLED=1

CMD ["npm", "run", "dev"]
