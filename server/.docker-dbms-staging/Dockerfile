# Staging image — CloudBeaver CE web UI for the staging Postgres database.
#
# CloudBeaver is a multi-platform database manager with a rich web UI
# (https://github.com/dbeaver/cloudbeaver). This image is a thin wrapper around
# the upstream `dbeaver/cloudbeaver` image: it pins a version, adds curl for
# the Docker HEALTHCHECK, and exposes the default port. There is no
# docker-compose for staging — the image is deployed standalone (ECS, Fly.io,
# Kubernetes, etc.) and connects to the managed Postgres through a connection
# the operator registers in the web UI on first launch.
#
# Usage:
#   docker build \
#     -f server/.docker-dbms-staging/Dockerfile \
#     -t charity-chest-dbms:staging \
#     server/.docker-dbms-staging
#
# Run (mount a persistent volume for the workspace — losing it discards all
# admin configuration, saved connections, and credentials):
#   docker run -d --name charity-chest-dbms \
#     -p 8978:8978 \
#     -v charity-chest-dbms-workspace:/opt/cloudbeaver/workspace \
#     charity-chest-dbms:staging
#
# Initial setup: open http://<host>:8978 once, walk through the quickstart
# wizard to create the admin user, then register the staging Postgres
# connection (host = staging Postgres endpoint, port 5432, database name + user
# from the staging credentials). The wizard runs only on first launch — every
# subsequent boot reads its configuration from the mounted workspace.
#
# Required runtime env vars: none — CloudBeaver is configured via the web
# wizard and the persistent workspace volume.
# Optional runtime env vars:
#   CLOUDBEAVER_WORKSPACE  Workspace path inside the container (default
#                          /opt/cloudbeaver/workspace). Only change this if you
#                          also adjust the volume mount target.
#
# Security: CloudBeaver has no built-in TLS. Always front it with a reverse
# proxy that terminates TLS (ALB, Cloud Run, Nginx, Caddy) and restrict access
# by VPN, IP allow-list, or SSO — the admin UI holds full DB credentials.

# Pin a specific CloudBeaver release. Update by checking
# https://hub.docker.com/r/dbeaver/cloudbeaver/tags and bumping this tag.
FROM dbeaver/cloudbeaver:26.0

# curl is used by the Docker HEALTHCHECK; the upstream image does not ship it.
RUN apt-get update && \
    apt-get install -y --no-install-recommends curl && \
    rm -rf /var/lib/apt/lists/*

# CloudBeaver listens on this port by default.
EXPOSE 8978

# start-period is generous — CloudBeaver loads driver metadata and applies
# workspace migrations on first cold start, which can take 30–60s.
HEALTHCHECK --interval=30s --timeout=5s --start-period=90s --retries=3 \
    CMD curl -fsS http://localhost:8978/ || exit 1
