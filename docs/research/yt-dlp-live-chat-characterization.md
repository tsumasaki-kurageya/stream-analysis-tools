# yt-dlp `2026.07.04` live-chat characterization

Date: 2026-08-12

Issue: [#11](https://github.com/tsumasaki-kurageya/stream-analysis-tools/issues/11)

## Outcome

Pin yt-dlp `2026.7.4` as the Worker dependency. The CLI reports
`2026.07.04`. The `live_chat` subtitle is a newline-delimited sequence of JSON
objects even though the filename ends in `.json`. Collection success must
require both exit code `0` and a completed artifact: a public archived stream
without chat returned `0` and produced no file.

The supporting primary-source review is in
[`yt-dlp-live-chat-primary-sources.md`](yt-dlp-live-chat-primary-sources.md).

## Pinned distribution

The Worker declares `yt-dlp==2026.7.4`; `uv.lock` records these hashes:

| Distribution | SHA-256                                                            |
| ------------ | ------------------------------------------------------------------ |
| PyPI wheel   | `f11f2b11d5a8ac4059f9bdf29fa4407dc7c6bb00c5097e95ca22a7a9db518266` |
| PyPI source  | `b094813404f87a9dd2186f00815231df32e5fd8a5403be0f807b3bb2d21a4432` |

The upstream release is
[`2026.07.04`](https://github.com/yt-dlp/yt-dlp/releases/tag/2026.07.04).
If deployment later uses a standalone binary rather than the locked Python
wheel, verify its name and hash against the release's signed
[`SHA2-256SUMS`](https://github.com/yt-dlp/yt-dlp/releases/download/2026.07.04/SHA2-256SUMS).

## Environment and command

- Windows 11 `10.0.26200`, x86-64
- Python `3.14.3`
- Deno `2.9.3`, Node.js `24.15.0`, FFmpeg `7.1.1`
- no cookies, account credentials, proxy, plugins, or remote components
- direct public YouTube access on the local development network

All three stream probes used the same argument shape:

```text
yt-dlp
--ignore-config
--no-plugin-dirs
--no-playlist
--skip-download
--write-subs
--sub-langs live_chat
--paths home:<case-dir>
--paths temp:<case-temp-dir>
--output subtitle:%(id)s.%(ext)s
--socket-timeout 30
--fragment-retries 2
--extractor-retries 2
--no-remote-components
<watch-url>
```

The output template remained relative. yt-dlp wrote the intermediate file
under the controlled `temp` directory, then moved the completed artifact to
`home/<video-id>.live_chat.json`. `--ignore-config --version` returned the
pinned version and exit code `0`; every empirical probe included the same
config-isolation flag.

## Stream results

Timings include YouTube metadata extraction and artifact acquisition.

| Case               | Public video                                                                                | Observed state                                       | Duration / offset range | Result                                                         |        Time | Artifact                     |
| ------------------ | ------------------------------------------------------------------------------------------- | ---------------------------------------------------- | ----------------------- | -------------------------------------------------------------- | ----------: | ---------------------------- |
| Short              | [`R3l34mHWmas`](https://www.youtube.com/watch?v=R3l34mHWmas), Nijisanji KR music stream     | replay advertised                                    | `0..194,375 ms`         | exit `0`; 533 NDJSON lines; 0 parse errors                     | `174.654 s` | `1,027,940 B`                |
| High volume        | [`I-J11Da5ONY`](https://www.youtube.com/watch?v=I-J11Da5ONY), Ninomae Ina'nis birthday live | `was_live`; `live_chat`; media duration `4,048 s`    | `0..4,049,533 ms`       | exit `0`; 54,389 NDJSON lines; 0 parse errors                  | `390.526 s` | `141,659,500 B`              |
| Replay unavailable | [`o8NiE3XMPrM`](https://www.youtube.com/watch?v=o8NiE3XMPrM), Google I/O '25 Keynote        | `was_live`; media duration `6,916 s`; no `live_chat` | n/a                     | exit `0`; “There are no subtitles for the requested languages” | `171.121 s` | no final or partial artifact |

The high-volume artifact contained these observed renderer counts:

| Renderer                                                 |  Count |
| -------------------------------------------------------- | -----: |
| `liveChatTextMessageRenderer`                            | 48,677 |
| `liveChatPlaceholderItemRenderer`                        |  3,778 |
| `liveChatSponsorshipsGiftRedemptionAnnouncementRenderer` |    577 |
| `liveChatMembershipItemRenderer`                         |    535 |
| `liveChatPaidMessageRenderer`                            |    119 |
| `liveChatSponsorshipsGiftPurchaseAnnouncementRenderer`   |     50 |
| `liveChatPaidStickerRenderer`                            |      6 |
| `liveChatViewerEngagementMessageRenderer`                |      1 |

It also contained 645 `addLiveChatTickerItemAction` actions and one
`addBannerToLiveChatCommand` action. Counts describe this run only; upstream
replay contents can change.

## Repeatability

The short case was downloaded twice into independent directories.

| Run |     Bytes | Lines | Offset range    | SHA-256                                                            |
| --- | --------: | ----: | --------------- | ------------------------------------------------------------------ |
| 1   | 1,027,940 |   533 | `0..194,375 ms` | `909bc0ed22eb2689caff3fe2508b633eecb680a5f8e1d4d33e734f8f37c32678` |
| 2   | 1,027,940 |   533 | `0..194,375 ms` | `89f298e63e25ea330ae1f6d38e506da584361d6a2d6b8d602012e285b231cb6c` |

Both runs parsed completely and had the same size, line count, and replay
range, but their raw hashes differed. Tests must therefore assert normalized
observable content rather than raw artifact identity.

## Timeout and exit behavior

`--socket-timeout` bounds an individual network operation, not the whole
process. A Worker-style outer deadline launched the pinned module against the
high-volume case, expired after five seconds, terminated it, and observed a
process return code of `1`, `5.009 s` elapsed, and no artifact. The Worker must
own a wall-clock deadline and process-tree termination; it must not interpret
that termination code as an upstream permanent failure.

Additional negative probes produced:

| Probe                                   | Exit | Time / artifact                       |
| --------------------------------------- | ---: | ------------------------------------- |
| invalid input `not-a-url`               |  `1` | `0.610 s`; no artifact                |
| unknown CLI option                      |  `2` | option parser rejected the invocation |
| accessible archived stream with no chat |  `0` | no artifact                           |

Do not classify outcomes from exit code alone. The collector should combine
the exit code, outer-timeout state, expected final-file presence, `.part`
presence, and line-by-line JSON validation.

## Artifact contract

For ended streams, yt-dlp `2026.07.04` exposed one subtitle entry named
`live_chat` with extension `json` and protocol `youtube_live_chat_replay`.
Each non-empty line was a standalone object centered on
`replayChatItemAction`, including `videoOffsetTimeMsec` and one or more action
objects. This is NDJSON, not a JSON array or object containing the entire
replay.

The committed fixtures under `tests/fixtures/yt-dlp-live-chat` retain the
observed renderer/action shapes. They replace all source identities, text,
timestamps, media URLs, tracking parameters, and monetary data. Raw artifacts
remain temporary and must not be committed.

## Decisions for #10 and #12

1. Treat exit `0` plus no final artifact as a no-chat/unavailable result.
2. Stream lines from disk; the high-volume artifact demonstrates why loading
   the document into memory is unacceptable.
3. Keep unknown renderers/actions countable and skippable.
4. Assert normalized database results and `CollectionResult`, not raw hashes,
   page counts, continuation values, or tracking parameters.
5. Give every attempt a controlled home/temp directory and remove it on every
   terminal path.
6. Enforce a Worker-owned wall-clock timeout and terminate the entire process
   tree before cleanup.
7. Re-run this characterization and the direct benchmark whenever the pinned
   yt-dlp version changes.
