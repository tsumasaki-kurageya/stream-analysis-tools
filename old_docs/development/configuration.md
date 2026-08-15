# Configuration and secrets

## Principles

- Application configuration is supplied through environment variables.
- Local values belong in `.env`; only `.env.example` is committed.
- Secrets, tokens, API keys, credentials, and production connection strings must never be committed.
- GitHub Actions secrets are used only when a workflow actually requires them.
- M0 does not introduce credentials for services that are not yet implemented.

## Naming

Project-owned variables use the `YSA_` prefix and an explicit responsibility-oriented name.

Examples:

```text
YSA_DATABASE_URL
YSA_YOUTUBE_API_KEY
```

Avoid ambiguous names such as `URL`, `TOKEN`, or `KEY`.

## Environments

- Local development: untracked `.env`
- Automated tests: isolated test configuration and disposable resources
- CI: workflow variables and secrets
- Production: deployment-platform configuration, defined when deployment is designed

Sample values must be clearly non-secret and must not resemble usable credentials.
