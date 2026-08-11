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

Install all dependencies:

```sh
make bootstrap
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
`migrations`, relative to the process working directory.

The default local connection is
`postgresql://stream_analysis:stream_analysis_local@localhost:5432/stream_analysis?sslmode=disable`.
Copy `.env.example` to `.env` to override local values. See the
[PostgreSQL development guide](docs/development/postgresql.md) for database conventions,
shutdown, reset, and troubleshooting procedures.

The Main API exposes stream metadata preview, registration, list, and detail endpoints.
Collection and reservation behavior are implemented by later milestone issues.
