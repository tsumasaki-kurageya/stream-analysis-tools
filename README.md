# Stream Analysis Tools

Stream Analysis Tools registers YouTube streams, collects archived chat, and supports synchronized exploration through a Web UI.

The initial architecture is defined in [ADR-0001](docs/adr/0001-system-architecture-and-yt-dlp-first-acquisition.md).

## Repository layout

| Path        | Purpose                                   |
| ----------- | ----------------------------------------- |
| apps/web    | React, TypeScript, and Vite Web UI        |
| apps/api    | Go Main API                               |
| apps/worker | Python Collection Worker                  |
| contracts   | Versioned external and internal contracts |
| migrations  | Ordered PostgreSQL migrations             |
| tests       | Cross-application and system tests        |
| docs        | Architecture and operating documentation  |

## Toolchains

- Node.js 24 LTS and npm 11
- Go 1.26
- Python 3.13 and uv

Exact local tool versions are recorded in `.tool-versions`. Each application also declares its own runtime constraints.

## Local development

### Dev Container

With Docker running, open the repository in a Dev Container-compatible editor and choose
**Reopen in Container**. The container installs the pinned Node.js, Go, Python, and uv versions
recorded in `.tool-versions`, plus Docker CLI and Compose support, PostgreSQL client tools,
ShellCheck, and GitHub CLI. It also runs `make bootstrap` after the container is created.

The container reuses the host Docker daemon, so the existing database and integration-test
commands work unchanged:

```sh
make db-up
make db-integration-test
```

Ports 5173 (Web UI), 8080 (Main API), and 5432 (PostgreSQL) are forwarded automatically.

### Manual setup

Install all dependencies:

```sh
make bootstrap
```

Copy the local environment file, set `YSA_YOUTUBE_API_KEY`, and start the full development stack:

```sh
cp -n .env.example .env
make dev
```

`make dev` starts PostgreSQL, builds and starts the Main API, enables and starts the Collection
Worker, and starts the Web UI. In a Dev Container it also handles the Docker network attachment.
Press Ctrl+C to stop the application processes, then stop PostgreSQL when it is no longer needed:

```sh
make dev-down
```

Run every formatting, lint, type-check, test, and build gate:

```sh
make check
```

Apply formatters:

```sh
make format
```

Each deployment unit remains independently operable:

```sh
make -C apps/web check build
make -C apps/api check build
make -C apps/worker check build
```

Start PostgreSQL and wait until it accepts connections:

```sh
make db-up
```

Start the Main API from the repository root after setting a YouTube Data API key in
the environment (or an untracked `.env` file loaded by your shell):

```sh
YSA_YOUTUBE_API_KEY=<api-key> ./apps/api/bin/main-api
```

The API applies ordered migrations at startup. `YSA_MIGRATIONS_DIR` defaults to
`migrations`, relative to the process working directory. It also runs the reservation monitor with
PostgreSQL leases; `YSA_RESERVATION_MONITOR_WORKER_ID` can set a stable instance identifier and
defaults to the host name.

The Worker checks its temporary-artifact directory at startup and before every collection attempt.
Queue consumption is fail-closed: `YSA_WORKER_QUEUE_ENABLED` defaults to `false` and must be set to
`true` explicitly only after the production canary gates pass. When disabled, the Worker remains
ready, handles shutdown signals, and never connects to or claims from PostgreSQL. `YSA_DATABASE_URL`
is required when queue consumption is enabled. `YSA_WORKER_ID`, `YSA_WORKER_LEASE_SECONDS`,
`YSA_WORKER_HEARTBEAT_INTERVAL_SECONDS`, `YSA_WORKER_POLL_INTERVAL_SECONDS`, and
`YSA_WORKER_ATTEMPT_TIMEOUT_SECONDS` configure claim ownership and bounded execution.
`YSA_WORKER_ATTEMPT_ROOT`, `YSA_WORKER_ORPHAN_AFTER_SECONDS`, and
`YSA_WORKER_MINIMUM_FREE_BYTES` configure its cleanup age and disk-capacity guard. Structured JSON
metrics and alert guidance are documented in the
[observability runbook](docs/operations/observability.md).
The release sequence, mandatory real-data cases, rollback steps, and evidence rules are documented in
the [production canary and rollback runbook](docs/operations/production-canary-and-rollback.md).

The default local connection is
`postgresql://stream_analysis:stream_analysis_local@localhost:5432/stream_analysis?sslmode=disable`.
Copy `.env.example` to `.env` to override local values. See the
[PostgreSQL development guide](docs/development/postgresql.md) for database conventions,
shutdown, reset, and troubleshooting procedures.

The Main API exposes stream metadata preview, registration, list, and detail endpoints plus collection
start/status/retry and stable chat pagination. The Collection Worker coordinates queued jobs through
PostgreSQL with exclusive claims, leases, heartbeats, and restart recovery. The API never accepts raw
yt-dlp, proxy, cookie, or output-path options and returns only allowlisted collection errors.
