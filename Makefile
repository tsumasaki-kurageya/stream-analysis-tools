SHELL := /usr/bin/env bash

.PHONY: bootstrap format format-check lint typecheck test build check clean

bootstrap:
	npm --prefix apps/web ci
	uv sync --project apps/worker --frozen --all-groups

format:
	$(MAKE) -C apps/web format
	$(MAKE) -C apps/api format
	$(MAKE) -C apps/worker format
	./apps/web/node_modules/.bin/prettier --write README.md ".github/**/*.yml" "{contracts,migrations,tests,docs}/**/*.md"

format-check:
	$(MAKE) -C apps/web format-check
	$(MAKE) -C apps/api format-check
	$(MAKE) -C apps/worker format-check
	./apps/web/node_modules/.bin/prettier --check README.md ".github/**/*.yml" "{contracts,migrations,tests,docs}/**/*.md"

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
