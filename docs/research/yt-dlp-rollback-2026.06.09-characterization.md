# yt-dlp `2026.06.09` rollback characterization

Date: 2026-08-15

Issue: [#22](https://github.com/tsumasaki-kurageya/stream-analysis-tools/issues/22)

## Outcome

yt-dlp `2026.6.9` is the characterized rollback release for the Worker pin
`2026.7.4`. This version is not the preferred production release. Its purpose is to provide a
known, immutable fallback when an incident is isolated to the current yt-dlp release.

The rollback target is the Git commit that contains this document and declares
`yt-dlp==2026.6.9` in `apps/worker/pyproject.toml`. Deploy that complete commit with its own
`uv.lock`; do not edit a running environment or use `yt-dlp -U`.

## Distribution identity

`2026.06.09` is the immediately preceding stable release before `2026.07.04` in the official
[yt-dlp release history](https://github.com/yt-dlp/yt-dlp/releases/tag/2026.06.09). PyPI records
Trusted Publishing provenance and these immutable hashes for
[`yt-dlp 2026.6.9`](https://pypi.org/project/yt-dlp/2026.6.9/):

| Distribution | SHA-256                                                            |
| ------------ | ------------------------------------------------------------------ |
| PyPI wheel   | `442ba4c75724b9496144c8434b617962ee08d0ee7c26ec663848fe9b78d5a3e4` |
| PyPI source  | `d50fcb95f48d61bedde33e408c1881d4c279e51c31354a599ce09e96ba0f4b86` |

The generated rollback `uv.lock` contained those exact hashes, `uv sync --frozen --all-groups`
installed the wheel, and `yt-dlp --ignore-config --version` returned `2026.06.09`.

## Environment and execution path

- Linux x86-64 development container
- Python `3.13.15`
- no cookies, account credentials, proxy, plugins, or remote components
- direct public YouTube access on the development network
- production `SubprocessYtDlpProcess` and `YtDlpChatReplayCollector`
- in-memory aggregate repository that retained only idempotency keys and batch sizes

The production adapter supplied its controlled argument array: config and plugins disabled, media
download skipped, only `live_chat` selected, separate controlled home/temp paths, bounded network
retries, no remote components, and a Worker-owned ten-minute deadline. Temporary attempt
directories were removed after every terminal outcome. No chat body or upstream stderr was
recorded.

## Canary results

| Case               | Public video  | Result          |     Duration | Aggregate evidence                                      |
| ------------------ | ------------- | --------------- | -----------: | ------------------------------------------------------- |
| Short replay       | `R3l34mHWmas` | succeeded       |    `4.878 s` | `1,027,940 B`; 511 saved; 22 skipped; maximum batch 500 |
| Replay unavailable | `o8NiE3XMPrM` | no data         |    `1.908 s` | no artifact; zero saved/skipped                         |
| Access denied      | `BEEgdCrsxdM` | bounded failure | not retained | safe `YTDLP_PROCESS_FAILED`; retryable; zero stored     |

The Worker format, lint, strict type-check, and non-integration test gates passed with this exact
pin: 27 tests passed and 26 Docker integration tests were deselected. The tests include controlled
argv, timeout/process-tree termination, artifact validation, bounded batches, and redaction
contracts.

## Rollback procedure

1. Disable queue consumption and stop all Worker replicas.
2. Deploy the complete characterized rollback commit, including its `pyproject.toml` and `uv.lock`.
3. Install with `uv sync --frozen`, verify version `2026.06.09`, and verify the selected artifact
   hash above.
4. Run the short, replay-unavailable, and access-denied canaries before enabling one Worker replica.
5. Keep the release on hold if any count, artifact, safe-error, cleanup, or timing contract changes.
6. Restore the current application commit after the incident, repeat the same canaries, and only
   then expand queue consumption.

## Limits

This is production-like local evidence, not a production deployment result. YouTube availability is
mutable, so the three public cases must be re-run in the target environment during an actual
rollback. Archive-not-ready behavior and the M1/M3/M4 browser paths still require a controlled
scheduled stream, target environment, and platform-managed YouTube Data API credential.
