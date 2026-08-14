from collections.abc import Sequence
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from pathlib import Path
from threading import Event
from typing import Protocol
from uuid import UUID

import pytest
from support.scripted_yt_dlp import ScriptedRun, ScriptedYtDlpProcess

from stream_analysis_worker.collector import (
    ChatReplayCollector,
    CollectionCancelled,
    CollectionErrorCode,
    CollectionFailure,
    CollectionOutcome,
    CollectionRequest,
    CollectionResult,
)
from stream_analysis_worker.yt_dlp_process import ProcessTermination

FIXTURE_ROOT = Path(__file__).parents[3] / "tests" / "fixtures" / "yt-dlp-live-chat"
BASIC_ARTIFACT = FIXTURE_ROOT / "basic.ndjson"
STREAM_ID = UUID("10000000-0000-0000-0000-000000000001")
FIRST_JOB_ID = UUID("20000000-0000-0000-0000-000000000001")
SECOND_JOB_ID = UUID("20000000-0000-0000-0000-000000000002")
RED_CONTRACT = pytest.mark.xfail(
    raises=NotImplementedError,
    strict=True,
    reason="Issue #12 implements the collector behind this interface contract",
)
pytestmark = pytest.mark.integration


@dataclass(frozen=True, slots=True)
class StoredChatMessage:
    external_message_id: str
    source: str
    author_channel_id: str | None
    author_display_name: str
    message_text: str
    published_at: datetime
    offset_milliseconds: int
    message_type: str


class CollectorContractHarness(Protocol):
    collector: ChatReplayCollector
    process: ScriptedYtDlpProcess
    attempt_root: Path
    batch_sizes: list[int]
    youtube_http_request_count: int

    def messages_for(self, stream_id: UUID) -> list[StoredChatMessage]: ...


def build_collector_contract_harness(
    *,
    attempt_root: Path,
    scripts: Sequence[ScriptedRun],
) -> CollectorContractHarness:
    """Issue #12 supplies the production collector and PostgreSQL-backed harness."""
    raise NotImplementedError


def request(*, job_id: UUID = FIRST_JOB_ID) -> CollectionRequest:
    return CollectionRequest(
        collection_job_id=job_id,
        stream_id=STREAM_ID,
        canonical_youtube_url="https://www.youtube.com/watch?v=fixture-video",
        attempt=1,
        deadline=datetime(2026, 8, 14, 12, 5, tzinfo=UTC),
    )


def assert_attempt_root_empty(attempt_root: Path) -> None:
    assert not attempt_root.exists() or list(attempt_root.iterdir()) == []


def write_large_artifact(path: Path, *, message_count: int) -> Path:
    template = BASIC_ARTIFACT.read_text(encoding="utf-8").splitlines()[1]
    lines = [
        template.replace("fixture-message-001", f"fixture-message-{index:04d}")
        for index in range(message_count)
    ]
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return path


@RED_CONTRACT
def test_success_returns_counts_and_persists_normalized_messages(tmp_path: Path) -> None:
    harness = build_collector_contract_harness(
        attempt_root=tmp_path,
        scripts=[ScriptedRun(artifact_source=BASIC_ARTIFACT)],
    )

    result = harness.collector.collect(request(), Event())

    assert result == CollectionResult(
        outcome=CollectionOutcome.SUCCEEDED,
        saved_message_count=1,
        duplicate_count=0,
        skipped_action_count=3,
        artifact_bytes=BASIC_ARTIFACT.stat().st_size,
        yt_dlp_version="2026.7.4",
        duration=timedelta(milliseconds=250),
    )
    assert harness.messages_for(STREAM_ID) == [
        StoredChatMessage(
            external_message_id="fixture-message-001",
            source="youtube_chat_replay",
            author_channel_id="fixture-channel-001",
            author_display_name="Fixture Author",
            message_text="fixture message text",
            published_at=datetime(2023, 11, 14, 22, 13, 21, tzinfo=UTC),
            offset_milliseconds=1000,
            message_type="text",
        )
    ]


@RED_CONTRACT
def test_exit_zero_without_an_artifact_returns_no_data(tmp_path: Path) -> None:
    harness = build_collector_contract_harness(
        attempt_root=tmp_path,
        scripts=[ScriptedRun()],
    )

    result = harness.collector.collect(request(), Event())

    assert result == CollectionResult(
        outcome=CollectionOutcome.NO_DATA,
        saved_message_count=0,
        duplicate_count=0,
        skipped_action_count=0,
        artifact_bytes=0,
        yt_dlp_version="2026.7.4",
        duration=timedelta(milliseconds=250),
    )
    assert harness.messages_for(STREAM_ID) == []


@RED_CONTRACT
def test_repeated_artifact_counts_duplicates_without_replacing_the_first_write(
    tmp_path: Path,
) -> None:
    harness = build_collector_contract_harness(
        attempt_root=tmp_path,
        scripts=[
            ScriptedRun(artifact_source=BASIC_ARTIFACT),
            ScriptedRun(artifact_source=BASIC_ARTIFACT),
        ],
    )

    first = harness.collector.collect(request(), Event())
    second = harness.collector.collect(request(job_id=SECOND_JOB_ID), Event())

    assert first.saved_message_count == 1
    assert second.saved_message_count == 0
    assert second.duplicate_count == 1
    assert second.skipped_action_count == 3
    assert [message.external_message_id for message in harness.messages_for(STREAM_ID)] == [
        "fixture-message-001"
    ]


@RED_CONTRACT
def test_malformed_artifact_is_a_safe_non_retryable_failure(tmp_path: Path) -> None:
    malformed = tmp_path / "malformed.ndjson"
    malformed.write_text('{"replayChatItemAction":', encoding="utf-8")
    harness = build_collector_contract_harness(
        attempt_root=tmp_path / "attempts",
        scripts=[ScriptedRun(artifact_source=malformed)],
    )

    with pytest.raises(CollectionFailure) as caught:
        harness.collector.collect(request(), Event())

    assert caught.value.code is CollectionErrorCode.YTDLP_OUTPUT_CHANGED
    assert caught.value.retryable is False
    assert caught.value.safe_message == "yt-dlp produced an unsupported chat artifact."
    assert "replayChatItemAction" not in str(caught.value)
    assert_attempt_root_empty(harness.attempt_root)


@RED_CONTRACT
def test_deadline_terminates_the_process_tree_and_returns_a_retryable_timeout(
    tmp_path: Path,
) -> None:
    harness = build_collector_contract_harness(
        attempt_root=tmp_path,
        scripts=[ScriptedRun(termination=ProcessTermination.TIMED_OUT)],
    )

    with pytest.raises(CollectionFailure) as caught:
        harness.collector.collect(request(), Event())

    assert caught.value.code is CollectionErrorCode.YTDLP_TIMEOUT
    assert caught.value.retryable is True
    assert harness.process.terminated_process_tree is True
    assert harness.messages_for(STREAM_ID) == []
    assert_attempt_root_empty(harness.attempt_root)


@RED_CONTRACT
def test_cancellation_terminates_the_process_tree_without_reporting_success(
    tmp_path: Path,
) -> None:
    harness = build_collector_contract_harness(
        attempt_root=tmp_path,
        scripts=[ScriptedRun(wait_for_cancellation=True)],
    )
    cancellation = Event()

    with ThreadPoolExecutor(max_workers=1) as executor:
        collecting = executor.submit(harness.collector.collect, request(), cancellation)
        assert harness.process.started.wait(timeout=2)
        cancellation.set()
        with pytest.raises(CollectionCancelled):
            collecting.result(timeout=2)

    assert harness.process.terminated_process_tree is True
    assert harness.messages_for(STREAM_ID) == []
    assert_attempt_root_empty(harness.attempt_root)


@RED_CONTRACT
def test_failure_redacts_stderr_credentials_paths_and_payloads(tmp_path: Path) -> None:
    secrets = (
        "cookie=secret-cookie",
        "https://user:password@proxy.example",
        "C:\\private\\job\\artifact.json",
        "continuation=raw-token",
        "private chat body",
    )
    harness = build_collector_contract_harness(
        attempt_root=tmp_path,
        scripts=[ScriptedRun(exit_code=1, stderr=" ".join(secrets))],
    )

    with pytest.raises(CollectionFailure) as caught:
        harness.collector.collect(request(), Event())

    assert caught.value.code is CollectionErrorCode.YTDLP_PROCESS_FAILED
    assert caught.value.safe_message == "yt-dlp failed while collecting chat replay."
    assert all(secret not in str(caught.value) for secret in secrets)
    assert_attempt_root_empty(harness.attempt_root)


@RED_CONTRACT
def test_collection_uses_one_process_no_youtube_http_and_bounded_batches(tmp_path: Path) -> None:
    artifact = write_large_artifact(tmp_path / "large.ndjson", message_count=501)
    harness = build_collector_contract_harness(
        attempt_root=tmp_path / "attempts",
        scripts=[ScriptedRun(artifact_source=artifact)],
    )

    result = harness.collector.collect(request(), Event())

    assert result.saved_message_count == 501
    assert harness.process.run_count == 1
    assert harness.youtube_http_request_count == 0
    assert harness.batch_sizes == [500, 1]


@pytest.mark.parametrize(
    ("script", "expected_exception"),
    [
        (ScriptedRun(artifact_source=BASIC_ARTIFACT), None),
        (ScriptedRun(), None),
        (ScriptedRun(termination=ProcessTermination.TIMED_OUT), CollectionFailure),
    ],
    ids=["success", "no-data", "timeout"],
)
@RED_CONTRACT
def test_attempt_directory_is_removed_after_terminal_paths(
    tmp_path: Path,
    script: ScriptedRun,
    expected_exception: type[Exception] | None,
) -> None:
    harness = build_collector_contract_harness(
        attempt_root=tmp_path,
        scripts=[script],
    )

    if expected_exception is None:
        harness.collector.collect(request(), Event())
    else:
        with pytest.raises(expected_exception):
            harness.collector.collect(request(), Event())

    assert_attempt_root_empty(harness.attempt_root)
