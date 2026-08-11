# Migrations

Ordered PostgreSQL migrations belong here.

Use a six-digit sequence and a snake-case description:

- `000001_create_streams.up.sql`
- `000001_create_streams.down.sql`

Up migrations are the production path. Down migrations support local and test recovery only and
must not be treated as the production rollback strategy. Bootstrap scripts under
`infrastructure/postgres/init/` create databases only; they must not apply product migrations.

Apply migrations to `POSTGRES_DB`. Automated integration tests must use `POSTGRES_TEST_DB` and
must never point at the primary local database.
