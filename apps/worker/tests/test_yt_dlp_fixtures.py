import json
from importlib.metadata import version
from pathlib import Path
from typing import Any

FIXTURE_ROOT = Path(__file__).parents[3] / "tests" / "fixtures" / "yt-dlp-live-chat"
SOURCE_IDENTIFIERS = ("R3l34mHWmas", "I-J11Da5ONY", "ytimg.com", "googleusercontent.com")


def test_yt_dlp_version_is_pinned() -> None:
    assert version("yt-dlp") == "2026.6.9"


def _strings(value: Any) -> list[str]:
    if isinstance(value, str):
        return [value]
    if isinstance(value, dict):
        return [text for item in value.values() for text in _strings(item)]
    if isinstance(value, list):
        return [text for item in value for text in _strings(item)]
    return []


def test_live_chat_fixtures_are_anonymized_ndjson() -> None:
    fixture_paths = sorted(FIXTURE_ROOT.glob("*.ndjson"))

    assert [path.name for path in fixture_paths] == ["basic.ndjson", "monetization.ndjson"]

    for path in fixture_paths:
        lines = path.read_text(encoding="utf-8").splitlines()
        assert lines
        for line in lines:
            record = json.loads(line)
            assert "replayChatItemAction" in record
            serialized_strings = _strings(record)
            assert not any(
                source_identifier in value
                for source_identifier in SOURCE_IDENTIFIERS
                for value in serialized_strings
            )
            assert all(
                value.startswith("https://example.invalid/")
                for value in serialized_strings
                if value.startswith(("http://", "https://"))
            )


def test_live_chat_fixtures_cover_observed_renderer_shapes() -> None:
    renderer_names: set[str] = set()
    action_names: set[str] = set()

    for path in FIXTURE_ROOT.glob("*.ndjson"):
        for line in path.read_text(encoding="utf-8").splitlines():
            record = json.loads(line)
            for action in record["replayChatItemAction"]["actions"]:
                action_names.update(action)
                item = action.get("addChatItemAction", {}).get("item", {})
                renderer_names.update(item)

    assert renderer_names == {
        "liveChatMembershipItemRenderer",
        "liveChatPaidMessageRenderer",
        "liveChatPaidStickerRenderer",
        "liveChatPlaceholderItemRenderer",
        "liveChatSponsorshipsGiftPurchaseAnnouncementRenderer",
        "liveChatSponsorshipsGiftRedemptionAnnouncementRenderer",
        "liveChatTextMessageRenderer",
        "liveChatViewerEngagementMessageRenderer",
    }
    assert action_names == {
        "addBannerToLiveChatCommand",
        "addChatItemAction",
        "addLiveChatTickerItemAction",
    }
