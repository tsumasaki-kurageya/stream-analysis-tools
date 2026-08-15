# Production canary and rollback

This runbook is the M4.1 release gate. Do not enable collection queue consumption until every
required gate has passed in the target production or production-equivalent environment. A local
rehearsal can validate mechanics, but it is not production completion evidence.

Never record database URLs, API keys, cookies, Authorization headers, proxy credentials, yt-dlp
stderr, or chat message bodies. Evidence may contain internal job/reservation UUIDs, stable error
codes, aggregate counts, durations, artifact byte counts, and tool versions.

## Required release inputs

Record these non-secret identifiers before deployment:

- release commit and immediately previous application commit;
- current and previously characterized yt-dlp versions and their lockfile hashes;
- target environment and deployment identifiers;
- one public short replay, one public high-volume replay, and one public replay-unavailable video;
- controlled access-denied and archive-not-ready cases whose owners permit testing.

If there is no previously characterized yt-dlp version, the yt-dlp rollback gate is **FAIL**. Do not
invent a rollback version or deploy an uncharacterized package.

The repository's characterized fallback is yt-dlp `2026.6.9` at application commit
`9b2df7ba39c2358b0ddf7827872b253824b108dc`. Its hashes, local canary evidence, and limitations are
recorded in the
[rollback characterization](../research/yt-dlp-rollback-2026.06.09-characterization.md).

## 1. Deploy fail-closed

Deploy the API and Worker from the same release commit with:

```text
YSA_WORKER_QUEUE_ENABLED=false
```

The Worker must emit `status=ready` and `queue_consumption=disabled` and remain running. Seed or
retain a queued canary job, wait longer than one poll interval, and verify its status and attempt stay
`queued` and `0`. Queue-disabled startup must not require database connectivity.

## 2. Preflight gates

All of these must pass before enabling the queue:

1. The API is ready and its `public.schema_migrations` ledger contains every committed `*.up.sql`
   migration exactly once. Never use down migrations as production rollback.
2. The installed `yt-dlp --version` equals the exact `apps/worker/pyproject.toml` pin. The selected
   wheel or sdist hash is present in `apps/worker/uv.lock`, and installation used `uv --frozen`.
3. Required credentials are present in the platform secret store. Report only `present` or `missing`.
4. API readiness succeeds through the same private route used by the Web application.
5. Worker startup emits `capacity_ok=true`; available bytes exceed
   `YSA_WORKER_MINIMUM_FREE_BYTES`; orphan cleanup completes without touching unrelated paths.
6. API and Worker logs pass the secret audit described in
   [observability.md](observability.md#redaction-contract).

## 3. Canary cases

Enable one canary Worker replica only after preflight passes:

```text
YSA_WORKER_QUEUE_ENABLED=true
```

Run cases sequentially. Record the source category, job ID, final public status, attempt count,
duration, aggregate counts, artifact bytes, yt-dlp version, and stable error code. Do not record chat
content.

| Case               | Required outcome                                                            |
| ------------------ | --------------------------------------------------------------------------- |
| Short replay       | succeeds; stored count equals saved count                                   |
| High-volume replay | succeeds within the release timeout and memory/batch gates                  |
| Replay unavailable | finishes as public `no_data` with no artifact                               |
| Access denied      | bounded failure with a safe allowlisted error; no upstream text leaks       |
| Archive not ready  | no premature collection; reservation remains waiting and retries monitoring |

After successful collection, exercise the M1-M4 Web path: preview/register, collection status, chat
list, player seek, search seek, reservation creation, automatic collection, and completed-reservation
navigation.

## 4. Restart and process termination

During a canary acquisition, record the running job ID and attempt, then terminate the Worker process.
For a hard-crash rehearsal use the platform equivalent of `SIGKILL`; for a graceful rehearsal use
`SIGTERM`. Confirm that:

- no second Worker claims before the lease expires;
- after expiry, a different stable Worker ID reclaims the same job with attempt incremented;
- the stale owner cannot heartbeat or finish;
- the recovered attempt reaches a terminal status;
- idempotent import prevents duplicate messages.

## 5. Application rollback

1. Set `YSA_WORKER_QUEUE_ENABLED=false` and confirm no new claims.
2. Gracefully stop Workers, then force-terminate any process that exceeds the shutdown allowance.
3. Deploy the immediately previous application commit without reverting forward migrations.
4. Verify API readiness and read existing streams, reservations, jobs, and chat aggregates.
5. Restore the release commit, repeat readiness, and keep queue consumption disabled until the
   incident decision is complete.

## 6. yt-dlp rollback

1. Keep queue consumption disabled.
2. Deploy the lockfile from the previously characterized yt-dlp release; never run `yt-dlp -U` or use
   a mutable download URL.
3. Verify the installed version and locked artifact hash.
4. Run short, replay-unavailable, and access-denied canaries.
5. Re-enable one replica only if all rollback canaries pass.

## 7. Completion report

Store a redacted Markdown report under `docs/operations/canary-reports/`. It must state PASS, FAIL,
or NOT RUN for every preflight, real-data, restart, secret, application rollback, and yt-dlp rollback
gate. Queue consumption may be enabled broadly only when the overall result is PASS. Outstanding
risks and the owner/next action are mandatory; an incomplete report is a release hold, not a waiver.
