SHELL := /usr/bin/env bash

.PHONY: bootstrap dev dev-down format format-check lint typecheck test build check clean \
	db-up db-wait db-status db-down db-reset db-logs db-smoke db-integration-test \
	contract-bootstrap contract-lint contract-generate contract-check benchmark-chat

bootstrap:
	npm --prefix apps/web ci
	npm --prefix contracts ci
	uv sync --project apps/worker --frozen --all-groups

dev:
	./scripts/dev.sh

dev-down: db-down

format:
	$(MAKE) -C apps/web format
	$(MAKE) -C apps/api format
	$(MAKE) -C apps/worker format
	./apps/web/node_modules/.bin/prettier --write README.md compose.yaml ".github/**/*.yml" "{contracts,migrations,tests,docs}/**/*.md"

format-check:
	$(MAKE) -C apps/web format-check
	$(MAKE) -C apps/api format-check
	$(MAKE) -C apps/worker format-check
	./apps/web/node_modules/.bin/prettier --check README.md compose.yaml ".github/**/*.yml" "{contracts,migrations,tests,docs}/**/*.md"

lint:
	$(MAKE) -C apps/web lint
	$(MAKE) -C apps/api lint
	$(MAKE) -C apps/worker lint

typecheck:
	$(MAKE) -C apps/web typecheck
	$(MAKE) -C apps/api typecheck
	$(MAKE) -C apps/worker typecheck

test:
	$(MAKE) -C apps/web test
	$(MAKE) -C apps/api test
	$(MAKE) -C apps/worker test

build:
	$(MAKE) -C apps/web build
	$(MAKE) -C apps/api build
	$(MAKE) -C apps/worker build

check: format-check lint typecheck test build

clean:
	$(MAKE) -C apps/web clean
	$(MAKE) -C apps/api clean
	$(MAKE) -C apps/worker clean

contract-bootstrap:
	npm --prefix contracts ci

contract-lint:
	npm --prefix contracts run format:check
	npm --prefix contracts run lint

contract-generate:
	npm --prefix contracts run generate
	$(MAKE) -C apps/api contract-generate

contract-check: contract-lint contract-generate
	git diff --exit-code -- apps/api/internal/generated/openapiv1/api.gen.go apps/web/src/api/generated/v1.ts
	test -z "$$(git status --porcelain -- apps/api/internal/generated/openapiv1/api.gen.go apps/web/src/api/generated/v1.ts)"

db-up:
	docker compose up --detach postgres
	./scripts/postgres-wait.sh

db-wait:
	./scripts/postgres-wait.sh

db-status:
	docker compose ps postgres

db-down:
	docker compose down

db-reset:
	docker compose down --volumes --remove-orphans
	docker compose up --detach postgres
	./scripts/postgres-wait.sh

db-logs:
	docker compose logs --follow postgres

db-smoke:
	./tests/postgres/compose-smoke.sh

db-integration-test:
	$(MAKE) -C apps/api integration-test
	$(MAKE) -C apps/worker integration-test

benchmark-chat:
	$(MAKE) -C apps/worker benchmark
