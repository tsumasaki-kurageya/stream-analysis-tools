# Repository structure

This document defines placement responsibilities without creating application scaffolding before it is needed.

```text
apps/                    # User-facing applications introduced from M1
  web/                   # React Web UI
  api/                   # Go Main API
workers/
  collector/             # Collection worker introduced from M2
packages/
  contracts/             # Shared API contracts when required
database/
  migrations/            # PostgreSQL migrations when persistence is introduced
tests/
  e2e/                   # End-to-end scenarios
docs/
  decisions/             # ADRs
  implementation-plans/  # Milestone implementation plans
.agents/
  skills/                # Repository-level Codex Agent Skills maintained separately
```

Directories are created when a milestone first needs them. Empty application projects and speculative shared packages must not be added during M0.
