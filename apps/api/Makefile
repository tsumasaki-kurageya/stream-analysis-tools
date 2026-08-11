.PHONY: format format-check lint typecheck test build check clean

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

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -o bin/main-api ./cmd/api

check: format-check lint typecheck test

clean:
	rm -rf bin
