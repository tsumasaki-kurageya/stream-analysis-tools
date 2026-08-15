#!/usr/bin/env bash
set -Eeuo pipefail

export COMPOSE_PROJECT_NAME="stream-analysis-tools-smoke"
export POSTGRES_USER="${POSTGRES_USER:-stream_analysis}"
export POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-stream_analysis_local}"
export POSTGRES_DB="${POSTGRES_DB:-stream_analysis}"
export POSTGRES_TEST_DB="${POSTGRES_TEST_DB:-stream_analysis_test}"
export POSTGRES_PORT="${POSTGRES_SMOKE_PORT:-55432}"

cleanup() {
  docker compose down --volumes --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

run_primary_sql() {
  docker compose exec -T postgres psql \
    --username "$POSTGRES_USER" \
    --dbname "$POSTGRES_DB" \
    --tuples-only \
    --no-align \
    --set=ON_ERROR_STOP=1 \
    "$@"
}

docker compose config --quiet
cleanup

docker compose up --detach --wait postgres

test_database_count="$(
  docker compose exec -T postgres psql \
    --username "$POSTGRES_USER" \
    --dbname postgres \
    --tuples-only \
    --no-align \
    --set=ON_ERROR_STOP=1 \
    --set=test_database="$POSTGRES_TEST_DB" <<'SQL'
SELECT count(*)
FROM pg_database
WHERE datname = :'test_database';
SQL
)"
test "$test_database_count" = "1"

run_primary_sql --command="CREATE TABLE issue_7_persistence_probe (value text PRIMARY KEY);"
run_primary_sql --command="INSERT INTO issue_7_persistence_probe (value) VALUES ('survives-restart');"

docker compose down
docker compose up --detach --wait postgres

persisted_value="$(
  run_primary_sql --command="SELECT value FROM issue_7_persistence_probe;"
)"
test "$persisted_value" = "survives-restart"

docker compose down --volumes
docker compose up --detach --wait postgres

probe_after_reset="$(
  run_primary_sql --command="SELECT to_regclass('public.issue_7_persistence_probe') IS NULL;"
)"
test "$probe_after_reset" = "t"

echo "PostgreSQL Compose smoke test passed."
