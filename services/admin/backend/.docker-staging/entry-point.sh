#!/bin/sh
# Best-effort: seed the first root user when ROOT_USER and ROOT_PASSWORD are
# both present. A failure (e.g. the user already exists) does not block the
# server from starting — seed-root's exit code is intentionally ignored.
if [ -n "${ROOT_USER}" ] && [ -n "${ROOT_PASSWORD}" ]; then
  ./seed-root -email "${ROOT_USER}" -password "${ROOT_PASSWORD}" || true
fi

exec ./server
