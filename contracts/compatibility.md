# HTTP v1 compatibility policy

The OpenAPI document is the source of truth for the main API. Product endpoints use the `/v1`
prefix. Operational endpoints such as `/healthz` are unversioned and follow the same additive-change
rules unless an infrastructure migration is explicitly coordinated.

## Compatible changes

- Add an endpoint, optional request field, optional response field, or non-required parameter.
- Add a response status without changing the meaning of an existing successful response.
- Relax a validation constraint while preserving the documented behavior.
- Deprecate an operation or field while keeping it functional throughout the announced migration
  window.

Consumers must ignore response fields they do not recognize. New request fields and parameters are
optional by default.

## Breaking changes

- Remove or rename an endpoint, field, parameter, media type, or response status.
- Make an optional input required or tighten an accepted value constraint.
- Change a field type, format, meaning, or success-response shape.
- Add an enum value when a consumer could reasonably use an exhaustive switch.
- Reuse a Problem Details `type` URI for a different problem.

Breaking product changes require a new major path such as `/v2`, or an explicitly documented
migration approved before the contract changes. A breaking operational change requires a coordinated
deployment plan.

## Error contract

HTTP errors use `application/problem+json` and the RFC 9457 `ProblemDetails` schema. Problem `type`
URIs are stable identifiers. Human-readable `title` and `detail` values are not machine-readable
codes. Extension members may add structured context without changing the five standard members.

## Change workflow

1. Edit `contracts/openapi/v1.yaml` before or together with implementation changes.
2. Run `make contract-generate` and commit both generated outputs.
3. Run `make contract-check`; CI repeats linting and regeneration and rejects drift.
4. Describe compatibility impact in the pull request. Treat any ambiguous change as breaking until
   reviewed.
