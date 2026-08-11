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

The scaffold intentionally contains only health/startup seams. Stream registration, persistence, collection, and reservation behavior are implemented by later milestone issues.
