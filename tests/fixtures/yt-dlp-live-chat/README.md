# yt-dlp live-chat fixtures

These NDJSON files preserve representative action and renderer shapes observed
with yt-dlp `2026.07.04`. They are intentionally small inputs for downstream
`ChatReplayCollector` parser tests.

The source artifacts are not committed. Author names, channel and message IDs,
message text, timestamps, image URLs, tracking parameters, and monetary values
were replaced with deterministic fixture values. URLs use the reserved
`example.invalid` domain.

- `basic.ndjson` covers viewer engagement, text messages, placeholders, and a
  banner command.
- `monetization.ndjson` covers membership, paid message, paid sticker, gift
  purchase/redemption, and ticker actions.

Each line is an independent JSON object because yt-dlp's `.live_chat.json`
artifact is JSON Lines/NDJSON rather than one JSON document.
