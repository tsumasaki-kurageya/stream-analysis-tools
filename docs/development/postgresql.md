# PostgreSQL local development

The local PostgreSQL service uses the official PostgreSQL 18.4 image, a named Docker volume, and a
health check based on `pg_isready`.

## Prerequisites

- Docker Engine or Docker Desktop with Docker Compose v2
- `make`

## Environment variables

| Variable               | Default                 | Purpose                                      |
| ---------------------- | ----------------------- | -------------------------------------------- |
| `POSTGRES_USER`        | `stream_analysis`       | Local database owner and login               |
| `POSTGRES_PASSWORD`    | `stream_analysis_local` | Local password; never use outside local work |
| `POSTGRES_DB`          | `stream_analysis`       | Primary development database                 |
| `POSTGRES_TEST_DB`     | `stream_analysis_test`  | Isolated integration-test database           |
| `POSTGRES_PORT`        | `5432`                  | Host port mapped to PostgreSQL               |
| `POSTGRES_INITDB_ARGS` | checksums and UTF-8     | Arguments used only on first initialization  |

Copy `.env.example` to `.env` only when an override is needed. Docker Compose loads `.env`
automatically, and `.env` is ignored by Git.

The readiness script also accepts the shell-only `POSTGRES_READY_TIMEOUT_SECONDS` variable, which
defaults to 60 seconds. Override it for one invocation with
`POSTGRES_READY_TIMEOUT_SECONDS=120 make db-up`.

## Start and readiness

Run one command from the repository root:

```sh
make db-up
```

This starts the `postgres` service and waits until `pg_isready` accepts connections to the primary
database. Inspect the current state with:

```sh
make db-status
```

Primary connection string:

```text
postgresql://stream_analysis:stream_analysis_local@localhost:5432/stream_analysis?sslmode=disable
```

Test connection string:

```text
postgresql://stream_analysis:stream_analysis_local@localhost:5432/stream_analysis_test?sslmode=disable
```

## Database conventions

- `POSTGRES_DB` is for developer data and manual application use.
- `POSTGRES_TEST_DB` is for automated integration tests. Tests must not target `POSTGRES_DB`.
- Files in `infrastructure/postgres/init/` bootstrap an empty volume only. They create database
  boundaries but do not apply product schema.
- Product schema changes live in `migrations/` and follow its ordered filename convention.
- Changing an initialization variable does not rewrite an existing volume. Run a clean reset when
  bootstrap variables change.

## Shutdown and persistence

Stop and remove the container and network while retaining the named data volume:

```sh
make db-down
```

The next `make db-up` reuses the volume, so local rows persist across container recreation.

## Backup-free local reset

> **Warning:** this permanently deletes the local PostgreSQL volume. It does not create a backup.

```sh
make db-reset
```

The command removes the volume, initializes a new primary database, recreates the test database,
and waits for readiness. Never use this target against shared or production infrastructure.

## Validation

The destructive smoke test uses its own Compose project name and host port. It proves initial
readiness, test-database creation, persistence after container recreation, and removal after a clean
reset:

```sh
make db-smoke
```

The test always removes its isolated volume on exit.

### Stream repository integration test

Run the migration and repository contract against an ephemeral PostgreSQL 18.4 container:

```sh
make db-integration-test
```

The Testcontainers test applies the ordered up migrations twice, verifies create, idempotent upsert,
unique YouTube video IDs, detail and newest-first list queries, and then verifies the down migration.
It requires a running Docker daemon and removes its container after the test.

## Troubleshooting

### Port 5432 is already in use

Set a different host port in `.env`, for example `POSTGRES_PORT=55432`, and run `make db-up` again.

### Readiness times out

Inspect recent logs:

```sh
docker compose logs --tail=100 postgres
```

Increase `POSTGRES_READY_TIMEOUT_SECONDS` for a slow machine. If initialization failed, correct the
configuration and run `make db-reset` because initialization scripts run only for an empty volume.

### Changed credentials do not work

The official image applies initialization credentials only to an empty data directory. Restore the
original `.env` values or run `make db-reset` if the local data can be discarded.

### Container exits unexpectedly

Run `make db-status` and `make db-logs`. Common causes are insufficient disk space, an invalid
`POSTGRES_INITDB_ARGS` value, or a volume created by an incompatible PostgreSQL major version.
