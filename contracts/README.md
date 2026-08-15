# Contracts

`openapi/v1.yaml` is the source of truth for the main API. It defines health, stream registration,
collection orchestration, stable chat pagination, and the RFC 9457 Problem Details error shape.

Install the pinned contract tooling and validate the document:

```sh
make contract-bootstrap
make contract-lint
```

Regenerate the Go server types and TypeScript client types after every contract change:

```sh
make contract-generate
```

Generated files are committed so application builds do not require generators. `make contract-check`
lints the contract, regenerates both outputs, and fails if the working tree changes. See
[`compatibility.md`](compatibility.md) before changing the v1 interface.
