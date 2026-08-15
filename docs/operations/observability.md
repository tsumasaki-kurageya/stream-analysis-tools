# Observability and temporary-artifact operations

The Main API and Collection Worker write one compact JSON object per metric event to standard output.
The events deliberately expose product identifiers, bounded numeric measurements, enum-like outcomes,
versions, and stable error codes only. They do not expose commands, URLs, local paths, exception text,
stdout/stderr, authorization or cookie values, proxy credentials, continuation values, or chat bodies.

## Event catalog

| Event                        | Component | Useful fields                                                                                                                                 |
| ---------------------------- | --------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `reservation_monitor_check`  | Main API  | `reservation_id`, `state`, `attempt`, `outcome`, `duration_seconds`, optional `error_code`                                                    |
| `collection_job`             | Worker    | `job_id`, `job_kind`, `attempt`, `outcome`, `duration_seconds`, processed/skipped counts, optional `error_code`                               |
| `yt_dlp_collection_attempt`  | Worker    | `job_id`, `attempt`, `outcome`, `duration_seconds`, saved/duplicate/skipped counts, `artifact_bytes`, `yt_dlp_version`, optional `error_code` |
| `temporary_artifact_cleanup` | Worker    | removed directory count and artifact bytes                                                                                                    |
| `disk_capacity`              | Worker    | free, used, and total bytes plus `capacity_ok`                                                                                                |

Convert event counts to counters and `duration_seconds`, `artifact_bytes`, and disk byte fields to
histograms or gauges in the log collector. Keep `job_id` and `reservation_id` available for traces but
do not use them as long-lived metric labels in systems where that would create unbounded cardinality.

## Dashboard

The service dashboard should show:

1. Collection jobs and yt-dlp attempts by outcome and stable error code, with retry attempt distribution.
2. p50, p95, and p99 reservation-check, job, and yt-dlp durations.
3. Saved, duplicate, and skipped messages; artifact bytes; and active yt-dlp version.
4. Temporary directories and bytes removed, disk free bytes, and the `capacity_ok` state.

Correlate `collection_job` and `yt_dlp_collection_attempt` by `job_id` only during incident analysis.
The version panel must make a mixed or unexpected yt-dlp rollout visible.

## Alerts

Start with these conditions and tune them from production baselines:

- Page immediately when any `disk_capacity` event has `capacity_ok=false`; warn when free bytes are less
  than twice `YSA_WORKER_MINIMUM_FREE_BYTES` for 15 minutes.
- Warn when failed yt-dlp attempts exceed 20% with at least five attempts in 15 minutes, grouped by
  `error_code`. Page on a sustained `YTDLP_OUTPUT_CHANGED` result because it can indicate an upstream
  artifact-contract change.
- Warn when reservation failures reach five attempts or `RESERVATION_PERSIST_FAILED` occurs repeatedly.
- Warn when p95 duration doubles its seven-day baseline for 30 minutes, and when orphan cleanup removes
  data on three consecutive runs. The latter often means a Worker is being terminated mid-attempt.
- Alert on more than one active `yt_dlp_version` outside a planned rollout window.

## Cleanup and capacity controls

`YSA_WORKER_ATTEMPT_ROOT` defaults to `/tmp/stream-analysis-worker`. Directories must match the
Worker-owned `<job-uuid>-attempt-<number>-<random>` shape before orphan cleanup will touch them. The
Worker removes only matching directories older than `YSA_WORKER_ORPHAN_AFTER_SECONDS` (24 hours by
default), then records removed bytes and a disk snapshot. Collection is refused when free space is below
`YSA_WORKER_MINIMUM_FREE_BYTES` (1 GiB by default). Every live attempt is also removed in a `finally`
path after success, no-data, failure, timeout, or cancellation.

## Redaction audit

For a release audit, run the Worker failure-path tests and inspect captured output/database assertions:

```sh
make -C apps/worker test
make -C apps/api test
```

Use synthetic sentinel strings for cookie, bearer authorization, proxy user/password, continuation,
chat-body, and raw-token values. Search test output and the affected job/reservation error fields for each
sentinel. The expected durable result is a stable error code plus a fixed product message; stderr is
represented only as `[REDACTED STDERR]`. Never perform this audit with real credentials or real chat.
