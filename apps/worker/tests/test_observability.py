import io
import json
import os
from datetime import UTC, datetime, timedelta
from pathlib import Path
from threading import Event
from uuid import UUID

import pytest

from stream_analysis_worker.chat_replay import YtDlpChatReplayCollector
from stream_analysis_worker.collector import (
    CollectionErrorCode,
    CollectionFailure,
    CollectionRequest,
)
from stream_analysis_worker.jobs import ClaimedJob, JobLease
from stream_analysis_worker.observability import (
    ArtifactDirectoryManager,
    CollectionAttemptMetric,
    JobMetric,
    JsonMetricSink,
    redact_text,
)
from stream_analysis_worker.worker import ClaimLoop, JobResult
from stream_analysis_worker.yt_dlp_process import YtDlpProcessRequest, YtDlpProcessResult


def test_collection_metric_contains_only_safe_operational_fields() -> None:
    output = io.StringIO()
    sink = JsonMetricSink(output)

    sink.emit(
        CollectionAttemptMetric(
            job_id="20000000-0000-0000-0000-000000000001",
            attempt=2,
            outcome="failed",
            duration_seconds=1.25,
            saved_message_count=0,
            duplicate_count=0,
            skipped_action_count=0,
            artifact_bytes=0,
            yt_dlp_version="2026.7.4",
            error_code="YTDLP_PROCESS_FAILED",
        )
    )

    assert json.loads(output.getvalue()) == {
        "artifact_bytes": 0,
        "attempt": 2,
        "duplicate_count": 0,
        "duration_seconds": 1.25,
        "error_code": "YTDLP_PROCESS_FAILED",
        "event": "yt_dlp_collection_attempt",
        "job_id": "20000000-0000-0000-0000-000000000001",
        "outcome": "failed",
        "saved_message_count": 0,
        "skipped_action_count": 0,
        "yt_dlp_version": "2026.7.4",
    }


def test_redaction_removes_credentials_and_opaque_values() -> None:
    unsafe = (
        "Authorization: Bearer auth-secret "
        "--cookies /tmp/private-cookies.txt "
        "--proxy https://proxy-user:proxy-password@proxy.example "
        "https://example.test/page?continuation=raw-continuation&token=raw-token"
    )

    redacted = redact_text(unsafe)

    for secret in (
        "auth-secret",
        "/tmp/private-cookies.txt",
        "proxy-user",
        "proxy-password",
        "raw-continuation",
        "raw-token",
    ):
        assert secret not in redacted
    assert redacted.count("[REDACTED]") >= 4


def test_artifact_manager_removes_only_orphaned_attempts_and_reports_disk(tmp_path: Path) -> None:
    output = io.StringIO()
    manager = ArtifactDirectoryManager(
        root=tmp_path,
        sink=JsonMetricSink(output),
        orphan_after=timedelta(hours=1),
        minimum_free_bytes=1,
    )
    old_attempt = tmp_path / "20000000-0000-0000-0000-000000000001-attempt-1-old"
    recent_attempt = tmp_path / "20000000-0000-0000-0000-000000000002-attempt-1-recent"
    unrelated = tmp_path / "keep-me"
    for directory in (old_attempt, recent_attempt, unrelated):
        directory.mkdir()
        (directory / "artifact").write_bytes(b"artifact")
    now = datetime(2026, 8, 15, 12, tzinfo=UTC)
    old_timestamp = (now - timedelta(hours=2)).timestamp()
    recent_timestamp = now.timestamp()
    os.utime(old_attempt, (old_timestamp, old_timestamp))
    os.utime(recent_attempt, (recent_timestamp, recent_timestamp))

    snapshot = manager.prepare(now=now)

    assert not old_attempt.exists()
    assert recent_attempt.exists()
    assert unrelated.exists()
    assert snapshot.removed_directory_count == 1
    assert snapshot.removed_artifact_bytes == len(b"artifact")
    events = [json.loads(line) for line in output.getvalue().splitlines()]
    assert [event["event"] for event in events] == [
        "temporary_artifact_cleanup",
        "disk_capacity",
    ]
    assert events[1]["free_bytes"] > 0


def test_collector_emits_safe_failure_metric_and_cleans_attempt(tmp_path: Path) -> None:
    output = io.StringIO()
    collector = YtDlpChatReplayCollector(
        process=FailingProcess(),
        messages=UnusedMessageRepository(),
        attempt_root=tmp_path,
        metric_sink=JsonMetricSink(output),
    )
    request = CollectionRequest(
        collection_job_id=UUID("20000000-0000-0000-0000-000000000001"),
        stream_id=UUID("10000000-0000-0000-0000-000000000001"),
        canonical_youtube_url="https://www.youtube.com/watch?v=fixture",
        attempt=3,
        deadline=datetime.now(UTC) + timedelta(minutes=1),
    )

    with pytest.raises(CollectionFailure) as caught:
        collector.collect(request, Event())

    assert caught.value.code is CollectionErrorCode.YTDLP_PROCESS_FAILED
    events = [json.loads(line) for line in output.getvalue().splitlines()]
    attempt = next(event for event in events if event["event"] == "yt_dlp_collection_attempt")
    assert attempt["job_id"] == str(request.collection_job_id)
    assert attempt["attempt"] == 3
    assert attempt["outcome"] == "failed"
    assert attempt["error_code"] == "YTDLP_PROCESS_FAILED"
    for secret in FailingProcess.secrets:
        assert secret not in output.getvalue()
    assert list(tmp_path.iterdir()) == []


def test_collector_classifies_low_disk_capacity_without_starting_process(
    tmp_path: Path,
) -> None:
    output = io.StringIO()
    sink = JsonMetricSink(output)
    collector = YtDlpChatReplayCollector(
        process=FailingProcess(),
        messages=UnusedMessageRepository(),
        attempt_root=tmp_path,
        metric_sink=sink,
        artifact_manager=ArtifactDirectoryManager(
            root=tmp_path,
            sink=sink,
            minimum_free_bytes=2**63 - 1,
        ),
    )
    request = CollectionRequest(
        collection_job_id=UUID("20000000-0000-0000-0000-000000000001"),
        stream_id=UUID("10000000-0000-0000-0000-000000000001"),
        canonical_youtube_url="https://www.youtube.com/watch?v=fixture",
        attempt=1,
        deadline=datetime.now(UTC) + timedelta(minutes=1),
    )

    with pytest.raises(CollectionFailure) as caught:
        collector.collect(request, Event())

    assert caught.value.code.value == "WORKER_DISK_CAPACITY_LOW"
    events = [json.loads(line) for line in output.getvalue().splitlines()]
    assert events[1]["event"] == "disk_capacity"
    assert events[1]["capacity_ok"] is False
    assert events[2]["event"] == "yt_dlp_collection_attempt"
    assert events[2]["error_code"] == "WORKER_DISK_CAPACITY_LOW"


def test_claim_loop_redacts_durable_error_and_emits_job_metric() -> None:
    output = io.StringIO()
    now = datetime(2026, 8, 15, 12, tzinfo=UTC)
    repository = RecordingJobRepository(now)
    loop = ClaimLoop(
        repository=repository,
        runner=FailingJobRunner(),
        worker_id="worker-1",
        lease_duration=timedelta(minutes=1),
        heartbeat_interval=timedelta(seconds=30),
        clock=lambda: now,
        metric_sink=JsonMetricSink(output),
    )

    assert loop.run_once(Event()) is True

    assert repository.error_code == "UPSTREAM_FAILED"
    assert repository.error_message == "Collection job failed."
    metric = json.loads(output.getvalue())
    assert metric["event"] == JobMetric.event
    assert metric["outcome"] == "failed"
    assert metric["error_code"] == "UPSTREAM_FAILED"
    assert "auth-secret" not in output.getvalue()


class FailingProcess:
    secrets = (
        "auth-secret",
        "secret-cookie",
        "proxy-user:proxy-password",
        "raw-continuation",
        "private-message-content",
    )

    def run(self, request: YtDlpProcessRequest, cancellation: Event) -> YtDlpProcessResult:
        del request, cancellation
        raise RuntimeError(" ".join(self.secrets))


class UnusedMessageRepository:
    def upsert_batch(self, messages: object) -> int:
        del messages
        raise AssertionError("messages must not be persisted")


class FailingJobRunner:
    def run(self, job: ClaimedJob, cancellation: Event) -> JobResult:
        del job, cancellation
        return JobResult.failed(
            error_code="UPSTREAM_FAILED",
            error_message=("Authorization: Bearer auth-secret chat body: private-message-content"),
        )


class RecordingJobRepository:
    def __init__(self, now: datetime) -> None:
        self.job = ClaimedJob(
            id=UUID("20000000-0000-0000-0000-000000000001"),
            stream_id=UUID("10000000-0000-0000-0000-000000000001"),
            canonical_youtube_url="https://www.youtube.com/watch?v=dQw4w9WgXcQ",
            kind="chat_replay",
            attempt=2,
            started_at=now,
            lease=JobLease(
                worker_id="worker-1",
                token=UUID("30000000-0000-0000-0000-000000000001"),
                heartbeat_at=now,
                expires_at=now + timedelta(minutes=1),
            ),
        )
        self.error_code: str | None = None
        self.error_message: str | None = None

    def claim_next(self, **kwargs: object) -> ClaimedJob:
        del kwargs
        return self.job

    def heartbeat(self, **kwargs: object) -> JobLease:
        del kwargs
        return self.job.lease

    def mark_succeeded(self, **kwargs: object) -> None:
        del kwargs
        raise AssertionError("job must fail")

    def mark_failed(self, **kwargs: object) -> None:
        self.error_code = str(kwargs["error_code"])
        message = kwargs["error_message"]
        self.error_message = None if message is None else str(message)
