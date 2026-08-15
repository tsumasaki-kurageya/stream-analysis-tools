.PHONY: sync format format-check lint typecheck test integration-test benchmark build check clean

BENCHMARK_VIDEO_ID ?= R3l34mHWmas
BENCHMARK_TIMEOUT_SECONDS ?= 1800
BENCHMARK_OUTPUT_DIRECTORY ?= dist/benchmarks

export BENCHMARK_DATABASE_URL

sync:
	uv sync --frozen --all-groups

format:
	uv run ruff format src tests
	uv run ruff check --fix src tests

format-check:
	uv run ruff format --check src tests

lint:
	uv run ruff check src tests

typecheck:
	uv run mypy src tests

test:
	uv run pytest -m "not integration"

integration-test:
	uv run pytest -m integration

benchmark:
	@test -n "$(BENCHMARK_DATABASE_URL)" || { \
		echo "BENCHMARK_DATABASE_URL is required" >&2; \
		exit 2; \
	}
	uv run python -m stream_analysis_worker.benchmark \
		--video-id "$(BENCHMARK_VIDEO_ID)" \
		--timeout-seconds "$(BENCHMARK_TIMEOUT_SECONDS)" \
		--json-output "$(BENCHMARK_OUTPUT_DIRECTORY)/direct-vs-worker.json" \
		--markdown-output "$(BENCHMARK_OUTPUT_DIRECTORY)/direct-vs-worker.md"

build:
	uv build

check: format-check lint typecheck test

clean:
	rm -rf dist .mypy_cache .pytest_cache .ruff_cache src/*.egg-info
