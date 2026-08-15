import json
import os
from collections.abc import Iterator, Sequence
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from pathlib import Path
from threading import Event
from typing import Any
from uuid import UUID

os.environ.setdefault("TESTCONTAINERS_RYUK_DISABLED", "true")

import psycopg
import pytest
from psycopg_pool import ConnectionPool
from support.scripted_yt_dlp import ScriptedRun, ScriptedYtDlpProcess
from testcontainers.community.postgres import PostgresContainer

from stream_analysis_worker.chat import PostgresChatMessageRepository
from stream_analysis_worker.chat_replay import YtDlpChatReplayCollector
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
MONETIZATION_ARTIFACT = FIXTURE_ROOT / "monetization.ndjson"
STREAM_ID = UUID("10000000-0000-0000-0000-000000000001")
FIRST_JOB_ID = UUID("20000000-0000-0000-0000-000000000001")
SECOND_JOB_ID = UUID("20000000-0000-0000-0000-000000000002")
MISSING_JOB_ID = UUID("20000000-0000-0000-0000-000000000099")
pytestmark = pytest.mark.integration


@pytest.fixture(scope="module")
def database_url() -> Iterator[str]:
    with PostgresContainer(
        image="postgres:18.4-bookworm",
        username="stream_analysis",
        password="stream_analysis_test",
        dbname="stream_analysis_test",
        driver=None,
    ) as postgres:
        url = postgres.get_connection_url(driver=None)
        with psycopg.connect(url, autocommit=True) as connection:
            for migration in sorted(migrations_directory().glob("*.up.sql")):
                connection.execute(migration.read_text(encoding="utf-8"))
        yield url


@pytest.fixture(autouse=True)
def clean_database(database_url: str) -> None:
    with psycopg.connect(database_url) as connection:
        connection.execute(
            "TRUNCATE chat.chat_messages, collection.collection_jobs, stream.streams CASCADE"
        )


@pytest.fixture
def pool(database_url: str) -> Iterator[ConnectionPool[Any]]:
    with ConnectionPool(database_url, min_size=1, max_size=2) as connection_pool:
        yield connection_pool


@dataclass(frozen=True, slots=True)
class StoredChatMessage:
    external_message_id: str
    collection_job_id: UUID
    source: str
    author_channel_id: str | None
    author_display_name: str
    message_text: str
    published_at: datetime
    offset_milliseconds: int
    message_type: str


@dataclass(slots=True)
class CollectorContractHarness:
    collector: ChatReplayCollector
    process: ScriptedYtDlpProcess
    attempt_root: Path
    batch_sizes: list[int]
    youtube_http_request_count: int
    pool: ConnectionPool[Any]

    def messages_for(self, stream_id: UUID) -> list[StoredChatMessage]:
        with self.pool.connection() as connection:
            rows = connection.execute(
                """
                SELECT
                    external_message_id,
                    collection_job_id,
                    source,
                    author_channel_id,
                    author_display_name,
                    message_text,
                    published_at,
                    offset_milliseconds,
                    message_type
                FROM chat.chat_messages
                WHERE stream_id = %s
                ORDER BY offset_milliseconds, published_at, external_message_id
                """,
                (stream_id,),
            ).fetchall()
        return [StoredChatMessage(*row) for row in rows]


def build_collector_contract_harness(
    *,
    pool: ConnectionPool[Any],
    attempt_root: Path,
    scripts: Sequence[ScriptedRun],
) -> CollectorContractHarness:
    seed_contract_records(pool)
    process = ScriptedYtDlpProcess(scripts)
    batch_sizes: list[int] = []
    messages = PostgresChatMessageRepository(pool, batch_observer=batch_sizes.append)
    return CollectorContractHarness(
        collector=YtDlpChatReplayCollector(
            process=process,
            messages=messages,
            attempt_root=attempt_root,
        ),
        process=process,
        attempt_root=attempt_root,
        batch_sizes=batch_sizes,
        youtube_http_request_count=0,
        pool=pool,
    )


def seed_contract_records(pool: ConnectionPool[Any]) -> None:
    with pool.connection() as connection:
        connection.execute(
            """
            INSERT INTO stream.streams (
                id,
                youtube_video_id,
                canonical_url,
                title,
                channel_id,
                channel_title,
                lifecycle_status,
                metadata_fetched_at
            ) VALUES (%s, 'fixture-video', %s, 'Fixture stream', 'fixture-channel',
                      'Fixture channel', 'ended', CURRENT_TIMESTAMP)
            """,
            (STREAM_ID, "https://www.youtube.com/watch?v=fixture-video"),
        )
        connection.execute(
            """
            INSERT INTO collection.collection_jobs (
                id,
                stream_id,
                kind,
                status,
                attempt,
                started_at,
                finished_at
            ) VALUES
                (%s, %s, 'chat_replay', 'succeeded', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
                (%s, %s, 'chat_replay', 'succeeded', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
            """,
            (FIRST_JOB_ID, STREAM_ID, SECOND_JOB_ID, STREAM_ID),
        )


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


def write_emoji_artifact(path: Path) -> Path:
    renderer = {
        "message": {
            "runs": [
                {"text": "hello "},
                {"emoji": {"emojiId": "wave", "shortcuts": [":wave:"]}},
            ]
        },
        "authorName": {"simpleText": "Fixture Author"},
        "id": "fixture-emoji-001",
        "timestampUsec": "1700000001500000",
        "authorExternalChannelId": "fixture-channel-001",
    }
    record = {
        "replayChatItemAction": {
            "actions": [{"addChatItemAction": {"item": {"liveChatTextMessageRenderer": renderer}}}],
            "videoOffsetTimeMsec": "1500",
        }
    }
    path.write_text(json.dumps(record) + "\n", encoding="utf-8")
    return path


def test_success_returns_counts_and_persists_normalized_messages(
    tmp_path: Path,
    pool: ConnectionPool[Any],
) -> None:
    harness = build_collector_contract_harness(
        pool=pool,
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
            collection_job_id=FIRST_JOB_ID,
            source="youtube_chat_replay",
            author_channel_id="fixture-channel-001",
            author_display_name="Fixture Author",
            message_text="fixture message text",
            published_at=datetime(2023, 11, 14, 22, 13, 21, tzinfo=UTC),
            offset_milliseconds=1000,
            message_type="text",
        )
    ]


def test_exit_zero_without_an_artifact_returns_no_data(
    tmp_path: Path,
    pool: ConnectionPool[Any],
) -> None:
    harness = build_collector_contract_harness(
        pool=pool,
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


def test_partial_artifact_is_not_classified_as_no_data(
    tmp_path: Path,
    pool: ConnectionPool[Any],
) -> None:
    harness = build_collector_contract_harness(
        pool=pool,
        attempt_root=tmp_path,
        scripts=[ScriptedRun(partial_artifact_present=True)],
    )

    with pytest.raises(CollectionFailure) as caught:
        harness.collector.collect(request(), Event())

    assert caught.value.code is CollectionErrorCode.YTDLP_OUTPUT_CHANGED
    assert caught.value.retryable is False
    assert_attempt_root_empty(harness.attempt_root)


def test_repeated_artifact_counts_duplicates_without_replacing_the_first_write(
    tmp_path: Path,
    pool: ConnectionPool[Any],
) -> None:
    harness = build_collector_contract_harness(
        pool=pool,
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
    assert harness.messages_for(STREAM_ID)[0].collection_job_id == FIRST_JOB_ID


def test_observed_unsupported_actions_are_counted_without_inventing_messages(
    tmp_path: Path,
    pool: ConnectionPool[Any],
) -> None:
    harness = build_collector_contract_harness(
        pool=pool,
        attempt_root=tmp_path,
        scripts=[ScriptedRun(artifact_source=MONETIZATION_ARTIFACT)],
    )

    result = harness.collector.collect(request(), Event())

    assert result.saved_message_count == 0
    assert result.duplicate_count == 0
    assert result.skipped_action_count == 6
    assert harness.messages_for(STREAM_ID) == []


def test_text_messages_preserve_emoji_shortcuts(
    tmp_path: Path,
    pool: ConnectionPool[Any],
) -> None:
    artifact = write_emoji_artifact(tmp_path / "emoji.ndjson")
    harness = build_collector_contract_harness(
        pool=pool,
        attempt_root=tmp_path / "attempts",
        scripts=[ScriptedRun(artifact_source=artifact)],
    )

    result = harness.collector.collect(request(), Event())

    assert result.saved_message_count == 1
    assert harness.messages_for(STREAM_ID)[0].message_text == "hello :wave:"


def test_malformed_artifact_is_a_safe_non_retryable_failure(
    tmp_path: Path,
    pool: ConnectionPool[Any],
) -> None:
    malformed = tmp_path / "malformed.ndjson"
    malformed.write_text('{"replayChatItemAction":', encoding="utf-8")
    harness = build_collector_contract_harness(
        pool=pool,
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


def test_deadline_terminates_the_process_tree_and_returns_a_retryable_timeout(
    tmp_path: Path,
    pool: ConnectionPool[Any],
) -> None:
    harness = build_collector_contract_harness(
        pool=pool,
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


def test_cancellation_terminates_the_process_tree_without_reporting_success(
    tmp_path: Path,
    pool: ConnectionPool[Any],
) -> None:
    harness = build_collector_contract_harness(
        pool=pool,
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


def test_failure_redacts_stderr_credentials_paths_and_payloads(
    tmp_path: Path,
    pool: ConnectionPool[Any],
) -> None:
    secrets = (
        "cookie=secret-cookie",
        "https://user:password@proxy.example",
        "C:\\private\\job\\artifact.json",
        "continuation=raw-token",
        "private chat body",
    )
    harness = build_collector_contract_harness(
        pool=pool,
        attempt_root=tmp_path,
        scripts=[ScriptedRun(exit_code=1, stderr=" ".join(secrets))],
    )

    with pytest.raises(CollectionFailure) as caught:
        harness.collector.collect(request(), Event())

    assert caught.value.code is CollectionErrorCode.YTDLP_PROCESS_FAILED
    assert caught.value.safe_message == "yt-dlp failed while collecting chat replay."
    assert all(secret not in str(caught.value) for secret in secrets)
    assert_attempt_root_empty(harness.attempt_root)


def test_database_failure_returns_only_safe_import_details(
    tmp_path: Path,
    pool: ConnectionPool[Any],
) -> None:
    harness = build_collector_contract_harness(
        pool=pool,
        attempt_root=tmp_path,
        scripts=[ScriptedRun(artifact_source=BASIC_ARTIFACT)],
    )

    with pytest.raises(CollectionFailure) as caught:
        harness.collector.collect(request(job_id=MISSING_JOB_ID), Event())

    assert caught.value.code is CollectionErrorCode.CHAT_IMPORT_FAILED
    assert caught.value.retryable is True
    assert caught.value.safe_message == "Chat messages could not be imported."
    assert str(MISSING_JOB_ID) not in str(caught.value)
    assert_attempt_root_empty(harness.attempt_root)


def test_collection_uses_one_process_no_youtube_http_and_bounded_batches(
    tmp_path: Path,
    pool: ConnectionPool[Any],
) -> None:
    artifact = write_large_artifact(tmp_path / "large.ndjson", message_count=5_001)
    harness = build_collector_contract_harness(
        pool=pool,
        attempt_root=tmp_path / "attempts",
        scripts=[
            ScriptedRun(artifact_source=artifact),
            ScriptedRun(artifact_source=artifact),
        ],
    )

    first = harness.collector.collect(request(), Event())
    second = harness.collector.collect(request(job_id=SECOND_JOB_ID), Event())

    assert first.saved_message_count == 5_001
    assert second.saved_message_count == 0
    assert second.duplicate_count == 5_001
    assert harness.process.run_count == 2
    assert harness.youtube_http_request_count == 0
    assert harness.batch_sizes == ([500] * 10 + [1]) * 2
    assert len(harness.messages_for(STREAM_ID)) == 5_001


@pytest.mark.parametrize(
    ("script", "expected_exception"),
    [
        (ScriptedRun(artifact_source=BASIC_ARTIFACT), None),
        (ScriptedRun(), None),
        (ScriptedRun(termination=ProcessTermination.TIMED_OUT), CollectionFailure),
    ],
    ids=["success", "no-data", "timeout"],
)
def test_attempt_directory_is_removed_after_terminal_paths(
    tmp_path: Path,
    pool: ConnectionPool[Any],
    script: ScriptedRun,
    expected_exception: type[Exception] | None,
) -> None:
    harness = build_collector_contract_harness(
        pool=pool,
        attempt_root=tmp_path,
        scripts=[script],
    )

    if expected_exception is None:
        harness.collector.collect(request(), Event())
    else:
        with pytest.raises(expected_exception):
            harness.collector.collect(request(), Event())

    assert_attempt_root_empty(harness.attempt_root)


def migrations_directory() -> Path:
    return Path(__file__).resolve().parents[3] / "migrations"
