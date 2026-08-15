.PHONY: format format-check lint typecheck test integration-test build check clean contract-generate

contract-generate:
	go tool oapi-codegen -config ../../contracts/oapi-codegen.yaml ../../contracts/openapi/v1.yaml

format:
	gofmt -w $$(find . -type f -name '*.go')

format-check:
	test -z "$$(gofmt -l $$(find . -type f -name '*.go'))"

lint:
	go vet ./...

typecheck:
	go test -run '^$$' ./...

test:
	go test ./...

integration-test:
	go test -tags=integration -count=1 -timeout=5m ./internal/...

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -o bin/main-api ./cmd/api

check: format-check lint typecheck test

clean:
	rm -rf bin
