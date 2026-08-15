# yt-dlp live chat: primary-source notes

Date: 2026-08-12

Scope: source research for Issue #11, against yt-dlp `2026.07.04` / Python package `2026.7.4`

This note intentionally separates facts documented by the projects that own the behavior from operational inferences and empirical work that still has to be run. YouTube and yt-dlp behavior is externally mutable, so a cited source is not a substitute for recording results from the pinned binary against the selected streams.

## Documented facts

### Version and release artifacts

- The current stable release inspected for this issue is [`2026.07.04`](https://github.com/yt-dlp/yt-dlp/releases/tag/2026.07.04), at source commit `fdec00e0bf530dc6c3cc7b1dd780e95d9ae460e9`.
- The repository currently pins the Python distribution as `yt-dlp==2026.7.4`. Its lock entry records the PyPI wheel SHA-256 as `f11f2b11d5a8ac4059f9bdf29fa4407dc7c6bb00c5097e95ca22a7a9db518266` and the source distribution SHA-256 as `b094813404f87a9dd2186f00815231df32e5fd8a5403be0f807b3bb2d21a4432`.
- If deployment switches to an upstream release executable, the official release checksum file records:
  - platform-independent `yt-dlp`: `495be29ff4d9d4e9be7eabdfef225221e5d5282e77f2f505abc6dca80349f3fd`;
  - glibc x86-64 `yt-dlp_linux`: `6bbb3d314cde4febe36e5fa1d55462e29c974f63444e707871834f6d8cc210ae`;
  - Windows x86-64 `yt-dlp.exe`: `52fe3c26dcf71fbdc85b528589020bb0b8e383155cfa81b64dd447bbe35e24b8`.
    See the official [`SHA2-256SUMS`](https://github.com/yt-dlp/yt-dlp/releases/download/2026.07.04/SHA2-256SUMS).
- yt-dlp publishes SHA-256 and SHA-512 lists, detached signatures, and a public key, with official verification examples in its [release-files documentation](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/README.md#release-files).

### Language tag and artifact shape

- yt-dlp treats an available live chat as a subtitle. The user-facing language tag is `live_chat`; the README uses that exact tag in `--sub-langs all,-live_chat` ([documented behavior](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/README.md#differences-in-default-behavior)).
- The YouTube extractor installs one entry under `subtitles['live_chat']`, assigns `ext: 'json'`, and distinguishes an active/scheduled stream from an ended replay only through internal protocols `youtube_live_chat` and `youtube_live_chat_replay` ([extractor source](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/yt_dlp/extractor/youtube/_video.py#L4378-L4390)). Thus both modes expose the same `live_chat` subtitle tag.
- Subtitle filenames are formed by adding the selected language and subtitle extension to the subtitle output base ([subtitle writer source](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/yt_dlp/YoutubeDL.py#L4450-L4469)). With a subtitle template of `%(id)s.%(ext)s`, the expected final name is therefore `<video-id>.live_chat.json`.
- Despite the `.json` suffix, the live-chat downloader writes one JSON object plus newline per action. Replay downloads write the upstream action; active-chat downloads wrap each action in a replay-compatible object containing `replayChatItemAction`, `videoOffsetTimeMsec`, and `isLive: true` ([downloader source](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/yt_dlp/downloader/youtube_live_chat.py#L40-L106)). The artifact is JSON Lines/NDJSON, not one JSON document.
- The downloader warns that an active live-chat download runs until the livestream ends ([downloader source](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/yt_dlp/downloader/youtube_live_chat.py#L17-L24)).

### Configuration and controlled paths

- `--ignore-config` stops loading further configuration files except paths explicitly supplied through `--config-locations`; the full configuration-loading caveat is documented in the [configuration section](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/README.md#configuration).
- `--write-subs` writes subtitle files, `--sub-langs` selects language tags, and `--list-subs` reports available tags ([subtitle options](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/README.md#subtitle-options)).
- `--skip-download` skips the media while still writing requested related files ([download options](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/README.md#download-options)).
- `--paths` supports typed paths plus `home` and `temp`. Intermediate files are written in `temp` and moved to `home` on completion. An absolute `--output` bypasses `--paths`, so the output template must remain relative if the Worker is to own the directory boundary ([filesystem options](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/README.md#filesystem-options)). `subtitle` is a supported typed output in the [source type table](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/yt_dlp/utils/_utils.py#L2877-L2889).
- Current YouTube support strongly recommends `yt-dlp-ejs` and a supported JavaScript runtime ([dependency documentation](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/README.md#strongly-recommended)). The runtime and remote-component policy used in characterization must therefore be recorded alongside yt-dlp itself.

A controlled characterization command should be represented as an argv array, for example:

```text
yt-dlp
--ignore-config
--no-playlist
--skip-download
--write-subs
--sub-langs live_chat
--paths home:<job-dir>
--paths temp:<job-temp-dir>
--output subtitle:%(id)s.%(ext)s
--socket-timeout 30
--fragment-retries 2
--extractor-retries 2
<youtube-watch-url>
```

`--no-plugin-dirs` is also appropriate when experiments must exclude locally installed plugins. The chosen JS runtime and `--remote-components`/`--no-remote-components` policy should be explicit rather than inherited from a machine.

### Timeout and retry behavior

- `--socket-timeout SECONDS` is documented as the time to wait before giving up on a network operation ([network options](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/README.md#network-options)). It is not documented as a process-wide or whole-download deadline.
- The live-chat downloader sleeps for a continuation-provided `timeoutMs` between active-chat requests and retries failed fragments through yt-dlp's fragment retry manager ([downloader source](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/yt_dlp/downloader/youtube_live_chat.py#L79-L133)).
- `--fragment-retries` controls fragment attempts and `--retry-sleep fragment:...` controls their delay; `--extractor-retries` is a separate option for known extraction errors ([download options](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/README.md#download-options), [extractor options](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/README.md#extractor-options)).

### Exit behavior

- yt-dlp initializes its download return code to zero. Reported errors set it to `1` when errors are being ignored; otherwise they raise `DownloadError` ([`YoutubeDL` source](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/yt_dlp/YoutubeDL.py#L1059-L1105)). `download()` returns that accumulated code ([source](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/yt_dlp/YoutubeDL.py#L3713-L3718)).
- The CLI maps cookie/download/unsafe-exec failures to `1`, option parsing failures to `2`, and internal `DownloadCancelled` to `101` ([CLI source](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/yt_dlp/__init__.py#L1064-L1092)).
- A missing requested subtitle is not necessarily a process error: the subtitle writer prints that no requested subtitles exist and returns without creating a file ([source](https://github.com/yt-dlp/yt-dlp/blob/2026.07.04/yt_dlp/YoutubeDL.py#L4450-L4460)). Exit `0` alone therefore does not prove that live chat was collected.

### YouTube-side availability

- YouTube says chat replay is enabled by default on stream archives, but the creator can turn it off ([YouTube Help](https://support.google.com/youtube/answer/9826490?hl=en#zippy=%2Cturn-off-live-chat-replay)).
- YouTube says any stream edited with the video editor will not have chat replay ([YouTube Help](https://support.google.com/youtube/answer/15268877?hl=en-GB)).
- Live chat and replay are disabled for made-for-kids streams, and YouTube may disable chat in other policy situations ([YouTube Help](https://support.google.com/youtube/answer/2853834?hl=en)). These are legitimate public-video/no-replay states rather than network failures.

## Operational inferences

These conclusions follow from the facts above but are not promises made by yt-dlp or YouTube:

1. Pin the Python package version and preserve the lockfile hashes. If deployment uses an executable instead, pin the exact tag, artifact name, and official checksum; never download from a mutable `releases/latest` URL and never run `-U` in a collection job.
2. Enforce a Worker-owned wall-clock deadline around the subprocess. `--socket-timeout` cannot bound a responsive but long-running chat download. Use a graceful terminate interval followed by kill, and record whether a `.part` file remains.
3. Treat success as all of: process exited `0`, the expected final `<id>.live_chat.json` exists under the controlled home path, it is not a `.part`, and every non-empty line parses as a JSON object. A zero exit with no artifact should become a distinct `chat_unavailable`/`no_chat` result, not `success`.
4. Do not parse free-form stderr to establish the primary outcome. Persist numeric exit code, timeout/termination cause, artifact state, and a bounded stderr excerpt for diagnostics.
5. Treat the JSON-line payload schema as version-scoped. The official source demonstrates the shape but does not promise cross-version stability; parser fixtures and an upgrade canary are required.
6. Store fixtures only after removing or irreversibly replacing author identifiers, handles, profile URLs/images, message IDs, and message text that is not essential to the parser case.

## Candidate streams

Candidates are not fixtures until the pinned command verifies their current availability. YouTube owners can delete videos, edit streams, or change replay settings later.

| Scenario                  | Candidate                                                                                                      | Why it is useful                                                                                                                           | Current empirical status                                                                                                                                      |
| ------------------------- | -------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Long / likely high volume | Google I/O '25 Keynote, video ID [`o8NiE3XMPrM`](https://www.youtube.com/watch?v=o8NiE3XMPrM)                  | First-party Google event, approximately 1 h 51 min, with millions of views; plausible high chat volume                                     | Public video page confirmed. `live_chat` listing/download not yet confirmed.                                                                                  |
| Short                     | Google, I/O '25 in under 10 minutes, video ID [`LxvErFkBXPk`](https://www.youtube.com/live/LxvErFkBXPk)        | First-party Google page and under ten minutes; suitable only if it was a Premiere/live event with retained replay                          | Public video page confirmed. Its origin and `live_chat` availability remain unverified.                                                                       |
| Replay unavailable        | Discover just-in-time from a public edited stream, a creator-disabled replay, or a public made-for-kids stream | These states are documented by YouTube and keep the video itself reachable, allowing `exit 0 but no artifact` behavior to be characterized | No durable video ID selected yet. A repository-owned unlisted test stream with replay disabled would be more deterministic than an external public candidate. |

The first two candidates were probed with the pinned Python package using `--list-subs`, explicit config isolation, bounded socket timeout, and zero extractor/fragment retries. In this environment the process did not finish within a 30-second outer deadline and was terminated, producing no usable listing. This is local empirical evidence of why an outer deadline is necessary; it is **not** evidence that either candidate has or lacks chat replay.

## Empirical matrix still required for Issue #11

For each accepted short, long/high-volume, and unavailable-replay stream, capture:

- UTC timestamp, watch URL/video ID, title/channel, and public availability state;
- `yt-dlp --version`, Python version, platform, JS runtime version, remote-component policy, full argv (with secrets redacted), and relevant environment;
- wall-clock deadline, socket timeout, retry counts/sleeps, observed process duration, numeric exit code, termination signal/reason, and bounded stdout/stderr;
- expected/final/partial artifact paths, byte size, line count, first/last replay offsets, and parse error count;
- `--list-subs` output showing whether `live_chat` was advertised;
- content-shape samples after anonymization: text, paid message/sticker, membership, deletion/retraction, ticker/banner, and unknown renderer when available;
- repeatability over at least two runs for the short case, and whether the output hash/line count changes.

Also run controlled negative probes for an invalid URL, an accessible video without chat, an inaccessible/deleted/private video, a socket stall where practical, and an outer-timeout termination. The exact exit codes and stderr wording for these cases must be recorded empirically rather than inferred from broad CLI mappings.
