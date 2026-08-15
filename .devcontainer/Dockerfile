ARG GO_VERSION=1.26.5
ARG UV_VERSION=0.11.33

FROM golang:${GO_VERSION}-bookworm AS go

FROM ghcr.io/astral-sh/uv:${UV_VERSION} AS uv

FROM mcr.microsoft.com/devcontainers/base:1-bookworm

COPY --from=go /usr/local/go /usr/local/go
COPY --from=uv /uv /uvx /usr/local/bin/

RUN apt-get update \
    && apt-get install --yes --no-install-recommends \
        make \
        postgresql-client \
        shellcheck \
    && mkdir -p /go \
    && chown -R vscode:vscode /go \
    && rm -rf /var/lib/apt/lists/*

ENV GOPATH=/go
ENV PATH=/usr/local/go/bin:/go/bin:${PATH}
