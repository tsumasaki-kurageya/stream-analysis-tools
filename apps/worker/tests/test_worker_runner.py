from datetime import UTC, datetime, timedelta
from threading import Event
from uuid import uuid4

from stream_analysis_worker.collector import (
    CollectionErrorCode,
    CollectionFailure,
    CollectionOutcome,
    CollectionRequest,
    CollectionResult,
)
from stream_analysis_worker.jobs import ClaimedJob, JobLease
from stream_analysis_worker.worker import ChatReplayJobRunner


def test_chat_replay_job_runner_maps_collection_result_to_job_result() -> None:
    now = datetime(2026, 8, 15, 12, tzinfo=UTC)
    collector = SuccessfulCollector()
    runner = ChatReplayJobRunner(
        collector=collector,
        clock=lambda: now,
        attempt_timeout=timedelta(minutes=30),
    )
    job = ClaimedJob(
        id=uuid4(),
        stream_id=uuid4(),
        canonical_youtube_url="https://www.youtube.com/watch?v=dQw4w9WgXcQ",
        kind="chat_replay",
        attempt=2,
        started_at=now,
        lease=JobLease(
            worker_id="worker",
            token=uuid4(),
            heartbeat_at=now,
            expires_at=now + timedelta(minutes=2),
        ),
    )

    result = runner.run(job, Event())

    assert result.processed_count == 12
    assert result.skipped_count == 3
    assert result.error_code is None
    assert collector.deadline == now + timedelta(minutes=30)


def test_chat_replay_job_runner_maps_safe_collection_failure_to_job_result() -> None:
    now = datetime(2026, 8, 15, 12, tzinfo=UTC)
    runner = ChatReplayJobRunner(
        collector=FailingCollector(),
        clock=lambda: now,
        attempt_timeout=timedelta(minutes=30),
    )

    result = runner.run(claimed_job(now), Event())

    assert result.processed_count == 0
    assert result.skipped_count == 0
    assert result.error_code == "YOUTUBE_ACCESS_DENIED"
    assert result.error_message == "The stream is not accessible."


def claimed_job(now: datetime) -> ClaimedJob:
    return ClaimedJob(
        id=uuid4(),
        stream_id=uuid4(),
        canonical_youtube_url="https://www.youtube.com/watch?v=dQw4w9WgXcQ",
        kind="chat_replay",
        attempt=2,
        started_at=now,
        lease=JobLease(
            worker_id="worker",
            token=uuid4(),
            heartbeat_at=now,
            expires_at=now + timedelta(minutes=2),
        ),
    )


class SuccessfulCollector:
    deadline: datetime | None = None

    def collect(self, request: CollectionRequest, cancellation: Event) -> CollectionResult:
        self.deadline = request.deadline
        assert cancellation.is_set() is False
        return CollectionResult(
            outcome=CollectionOutcome.SUCCEEDED,
            saved_message_count=12,
            duplicate_count=4,
            skipped_action_count=3,
            artifact_bytes=100,
            yt_dlp_version="2026.7.4",
            duration=timedelta(seconds=2),
        )


class FailingCollector:
    def collect(self, request: CollectionRequest, cancellation: Event) -> CollectionResult:
        raise CollectionFailure(
            code=CollectionErrorCode.YOUTUBE_ACCESS_DENIED,
            retryable=False,
            safe_message="The stream is not accessible.",
        )
