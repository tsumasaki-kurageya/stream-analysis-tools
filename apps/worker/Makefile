.PHONY: sync format format-check lint typecheck test integration-test build check clean

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

build:
	uv build

check: format-check lint typecheck test

clean:
	rm -rf dist .mypy_cache .pytest_cache .ruff_cache src/*.egg-info
