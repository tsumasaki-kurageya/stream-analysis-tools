# Local development

## Recommended path: Dev Container

Prerequisites:

- Docker-compatible container runtime
- Visual Studio Code with Dev Containers, or another compatible client

Open the repository in the Dev Container. The container installs Node.js, Go, Python, Docker CLI support, PostgreSQL client tools, GitHub CLI, Make, ShellCheck, and Graphify.

`make setup` creates a repository-local `.venv` for the Worker and YouTube Data Gateway. Python commands in the Makefile use this environment automatically; do not install their packages into the system Python.

After creation, verify the environment:

```bash
node --version
go version
python3 --version
docker version
psql --version
gh --version
graphify --version
make check
```

## M1 startup

Prepare local settings and dependencies:

```bash
cp .env.example .env
make setup
```

The Make targets load `.env` automatically. Do not source the file or export the
same settings manually before running them.

Start PostgreSQL, Main API, Web UI, YouTube Data Gateway, and Worker together:

```bash
make dev
```

Open the Web UI at `http://localhost:5173`. The page calls `GET /api/health` through the Vite proxy and displays whether the Main API is reachable.

Service endpoints:

- Web UI: `http://localhost:5173`
- Main API health: `http://localhost:8080/api/health`
- PostgreSQL: `localhost:5432`

Stop the foreground Web/API processes with Ctrl+C, then stop PostgreSQL with:

```bash
make dev-stop
```

PostgreSQL data remains in the named Compose volume. Use `docker compose down -v` only when a clean database is explicitly required.

## Common commands

```bash
make setup      # Install Web, Go, and Python dependencies
make db-up      # Start PostgreSQL only
make api        # Start Main API only
make web        # Start Web UI only
make dev        # Start PostgreSQL, Main API, and Web UI
make dev-stop   # Stop Compose services
make dev-logs   # Follow PostgreSQL logs
make test       # Run available tests
make lint       # Run formatting and type checks
make format     # Format Go source
make check      # Run the CI-equivalent validation entry point
make graphify        # Rebuild the local, code-only knowledge graph
make graphify-update # Update code nodes in an existing graph
```

## Graphify

The Dev Container installs the `graphifyy` package at version `0.9.30`. Its `graphify` command is available on `PATH`, and the project-scoped Codex Skill is stored in `.codex/skills/graphify/`.

`make graphify` uses deterministic local AST extraction by default:

```bash
make graphify
```

This default passes `--code-only`, so source code stays local and no LLM credentials are required. To include Markdown and other semantic content from a Codex session, invoke the project Skill with `$graphify .`.

After source-code changes, update an existing graph with:

```bash
make graphify-update
```

Graphify writes generated files under `graphify-out/`. The directory is currently Git-ignored. Whether those files should be committed will be decided after the proof of concept; remove the ignore rule only after that decision.

No Graphify Git hooks, Neo4j integration, MCP server, watch mode, or CI graph generation are enabled.

## Troubleshooting

- Dev Container build fails: rebuild without cache and confirm access to container registries and language download hosts.
- Web displays `未接続`: confirm `make api` is running and `curl http://localhost:8080/api/health` succeeds.
- PostgreSQL does not become healthy: inspect `make dev-logs` and confirm port 5432 is unused.
- Docker CLI cannot connect: confirm the host Docker daemon is running and Docker-outside-of-Docker is available.
- `graphify` is unavailable or reports another version: rebuild the Dev Container.
- `make graphify-update` reports a missing graph: run `make graphify` first.
- `make check` reports a missing file: restore the required artifact or update validation only when the convention changes intentionally.
- GitHub CLI is unauthenticated: run `gh auth login` before publishing branches or pull requests from the container.
