# Direct yt-dlp vs Worker benchmark

This benchmark is the M2 performance gate for archived YouTube chat collection. It runs
direct yt-dlp acquisition and the Worker collector sequentially against the same public stream,
on the same machine and network, with the repository's pinned yt-dlp version.

It is intentionally excluded from `make check` and normal CI because it uses live YouTube data
and its duration depends on the network. Run it for a release candidate and whenever the pinned
yt-dlp version changes.

## Fixed conditions

- Video ID: `R3l34mHWmas`, the short public archived stream characterized in
  [the yt-dlp spike](../research/yt-dlp-live-chat-characterization.md).
- Credentials: none. The production adapter does not accept cookie, proxy, or credential flags.
- Order: direct yt-dlp first, then one Worker collection attempt.
- Database: a migrated disposable or local PostgreSQL database. The benchmark inserts records with
  unique IDs and removes its messages, job, and stream after measurement.
- Platform: Linux with `/proc`, used to sample peak resident memory for each process tree.

If the fixed stream becomes unavailable, select another short public archived stream with chat,
update both this document and the characterization record, and use the same replacement ID for both
measurements.

## Run

Start PostgreSQL and ensure all migrations have been applied (starting the Main API once applies
them). Then export the connection string; it is read from the environment and is not written to the
report or echoed by the benchmark command.

```sh
make db-up
export BENCHMARK_DATABASE_URL='postgresql://stream_analysis:stream_analysis_local@localhost:5432/stream_analysis?sslmode=disable'
make benchmark-chat
```

Optional Make variables are `BENCHMARK_VIDEO_ID`, `BENCHMARK_TIMEOUT_SECONDS`, and
`BENCHMARK_OUTPUT_DIRECTORY`. The default reports are written under
`apps/worker/dist/benchmarks/`, which is ignored by Git. To attach durable evidence to an issue or
release, copy both generated files without editing them.

Raw live-chat artifacts remain in temporary directories and are deleted after each measurement.
The benchmark records no chat contents, credentials, connection string, or yt-dlp stderr.

## Measurements and gates

The JSON and Markdown reports record acquisition, import, and total wall time; artifact bytes; peak
RSS; yt-dlp process counts; saved, duplicate, skipped, and stored message counts; and the maximum
database batch size. Direct yt-dlp performs no import, so its import time is zero and normalized
message counts are not applicable.

All of these gates must pass:

1. Direct and Worker runs report the same pinned yt-dlp version.
2. Worker total time is at most `direct total × 1.25 + 60 seconds`.
3. Worker peak RSS is less than 512 MiB.
4. Direct and Worker each start exactly one yt-dlp process.
5. Worker-owned YouTube HTTP requests remain zero.
6. The stored row count equals the Worker's saved message count.
7. No import batch exceeds 500 messages.

The zero-HTTP assertion is structural: the measured Worker path composes only the yt-dlp subprocess
adapter and PostgreSQL repository. The collector contract test separately fixes that same boundary.
yt-dlp's own requests are expected and belong to its child process.

## Verified result

The latest repository result is the 2026-08-15 run for yt-dlp `2026.7.4`:

- [Human-readable report](results/2026-08-15-yt-dlp-2026.7.4-R3l34mHWmas.md)
- [Machine-readable report](results/2026-08-15-yt-dlp-2026.7.4-R3l34mHWmas.json)
