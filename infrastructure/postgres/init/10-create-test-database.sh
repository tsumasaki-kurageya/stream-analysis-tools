#!/usr/bin/env bash
set -Eeuo pipefail

test_database="${POSTGRES_TEST_DB:?POSTGRES_TEST_DB is required}"
database_owner="${POSTGRES_USER:?POSTGRES_USER is required}"

psql \
  --username "$POSTGRES_USER" \
  --dbname postgres \
  --set=ON_ERROR_STOP=1 \
  --set=test_database="$test_database" \
  --set=database_owner="$database_owner" <<'SQL'
SELECT format('CREATE DATABASE %I OWNER %I', :'test_database', :'database_owner')
WHERE NOT EXISTS (
  SELECT 1
  FROM pg_database
  WHERE datname = :'test_database'
) \gexec
SQL
