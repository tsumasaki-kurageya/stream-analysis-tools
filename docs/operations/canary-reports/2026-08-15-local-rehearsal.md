# M4.1 local canary and rollback rehearsal — 2026-08-15

- Overall result: **HOLD — production gates incomplete**
- Base commit: `97122fa4eca3c7a32e2d34b5f36f1b983142fc6a`
- Environment: isolated Docker network with PostgreSQL 18.4; Linux Worker container
- Credentials recorded: none; the API used a non-secret placeholder and no metadata request
- Queue after rehearsal: disabled; all canary Worker containers stopped

This is production-like local evidence, not a production deployment report. No database URL, API
key, cookie, proxy value, yt-dlp stderr, or chat body is included.

## Preflight

| Gate                     | Result               | Evidence                                                                                               |
| ------------------------ | -------------------- | ------------------------------------------------------------------------------------------------------ |
| queue initially disabled | PASS                 | queued job remained `queued`, attempt `0`; disabled Worker stayed ready and exited `0` on SIGTERM      |
| migrations               | PASS                 | isolated database applied all 6 committed migrations; API health returned `ok`                         |
| yt-dlp pin and checksum  | PASS                 | installed `2026.7.4`; lock wheel SHA-256 `f11f2b…518266`, sdist SHA-256 `b09481…a4432`; `uv --frozen`  |
| credentials              | NOT RUN              | no production secret store or YouTube Data API credential was available                                |
| readiness                | PASS (local)         | current API and rollback API returned `{"component":"main-api","status":"ok"}` on the private network  |
| temporary capacity       | PASS                 | 937,415,946,240 free bytes reported; cleanup removed no unrelated path                                 |
| secret audit             | PASS (observed logs) | emitted records contained only IDs, counts, durations, versions, stable codes, and capacity aggregates |

## Real-data collection

| Case                             | Result  | Evidence                                                                                                                         |
| -------------------------------- | ------- | -------------------------------------------------------------------------------------------------------------------------------- |
| short replay `R3l34mHWmas`       | PASS    | attempt 1; 4.44 s job time; 1,027,940-byte artifact; 511 saved; 22 skipped                                                       |
| high-volume replay `I-J11Da5ONY` | PASS    | attempt 1; 203.07 s job time; 141,659,603-byte artifact; 48,677 saved; 5,712 skipped                                             |
| replay unavailable `o8NiE3XMPrM` | PASS    | attempt 1; 1.57 s; no artifact; zero saved/skipped; public status maps to `no_data`                                              |
| unavailable `aaaaaaaaaaa`        | PASS    | current pin recheck: bounded failure in 1.81 s; safe non-retryable `CHAT_REPLAY_NOT_AVAILABLE`; upstream text absent; temp clean |
| access denied `BEEgdCrsxdM`      | PASS    | current pin recheck: bounded failure in 1.81 s; safe non-retryable `YOUTUBE_ACCESS_DENIED`; upstream text absent; temp clean     |
| archive not ready                | NOT RUN | public live candidate no longer produced the expected live/not-ready state; deterministic controlled case is still required      |

The short case ran again after recovery and reported all 511 messages as duplicates, leaving the
stored count unchanged. The high-volume stored total plus short total was 49,188 messages.

## Restart and rollback

| Gate                         | Result       | Evidence                                                                                                                                          |
| ---------------------------- | ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| hard Worker restart          | PASS         | Worker killed at `running`, attempt 1, exit 137; lease remained active until expiry                                                               |
| lease recovery               | PASS         | second Worker reclaimed the same job at attempt 2 and succeeded                                                                                   |
| idempotent recovery          | PASS         | recovered attempt saved 0, counted 511 duplicates, and retained 511 stored messages                                                               |
| graceful process termination | PASS         | idle enabled/disabled Workers stopped with exit 0                                                                                                 |
| application rollback         | PASS (local) | API from base commit started against the 6-migration DB; health `ok`; 7 jobs and 49,188 messages remained readable; current API restored health   |
| yt-dlp version rollback      | PASS (local) | rollback commit `9b2df7b` installed frozen `2026.6.9`; short/no-data/access-denied canaries passed; current `2026.7.4` pin and lock were restored |

## M1–M4 and production evidence

| Gate                              | Result       | Reason                                                                                                        |
| --------------------------------- | ------------ | ------------------------------------------------------------------------------------------------------------- |
| M1 real metadata preview/register | NOT RUN      | production YouTube Data API credential and target were unavailable                                            |
| M2 real collection/performance    | PASS (local) | short and high-volume cases above; prior direct-vs-Worker benchmark also passed                               |
| M3 real player/search/seek        | NOT RUN      | no production Web target was available                                                                        |
| M4 real reservation-to-completion | NOT RUN      | no controlled upcoming stream and production Web/API target were available                                    |
| production deployment             | NOT RUN      | no deployment target, platform credentials, or service configuration exists in the current repository/session |

## Outstanding risks and next actions

1. **Release owner:** provide the production or production-equivalent target and platform access;
   deploy current API/Web/Worker with queue disabled and repeat this report there.
2. **Release owner:** provide a scoped YouTube Data API credential through the platform secret store;
   run the M1 and M4 browser paths without copying the value into evidence.
3. **Release owner:** repeat the characterized yt-dlp `2026.6.9` rollback in the target environment;
   local evidence does not replace the production gate.
4. **QA owner:** select a controlled scheduled/live stream and complete archive-not-ready plus full
   reservation-to-collection evidence.

Queue consumption must remain disabled outside this isolated rehearsal until these blockers close.
