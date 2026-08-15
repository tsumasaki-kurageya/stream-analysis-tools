#!/usr/bin/env bash
set -Eeuo pipefail

timeout_seconds="${POSTGRES_READY_TIMEOUT_SECONDS:-60}"
deadline=$((SECONDS + timeout_seconds))

while ! docker compose exec -T postgres \
  sh -ec 'pg_isready --username="$POSTGRES_USER" --dbname="$POSTGRES_DB"' >/dev/null 2>&1; do
  if ((SECONDS >= deadline)); then
    echo "PostgreSQL did not become ready within ${timeout_seconds} seconds." >&2
    docker compose ps postgres >&2 || true
    docker compose logs --tail=100 postgres >&2 || true
    exit 1
  fi
  sleep 1
done

echo "PostgreSQL is ready."
