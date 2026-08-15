from collections.abc import Callable
from dataclasses import dataclass
from datetime import datetime, timedelta
from threading import Event, Lock, Thread
from time import monotonic
from typing import Protocol
from uuid import UUID

from stream_analysis_worker.collector import (
    ChatReplayCollector,
    CollectionFailure,
    CollectionRequest,
)
from stream_analysis_worker.jobs import ClaimedJob, JobLease
from stream_analysis_worker.observability import (
    JobMetric,
    MetricSink,
    NullMetricSink,
    safe_error_code,
)


@dataclass(frozen=True, slots=True)
class JobResult:
    processed_count: int
    skipped_count: int
    error_code: str | None = None
    error_message: str | None = None

    @classmethod
    def succeeded(cls, *, processed_count: int, skipped_count: int) -> "JobResult":
        return cls(processed_count=processed_count, skipped_count=skipped_count)

    @classmethod
    def failed(cls, *, error_code: str, error_message: str | None) -> "JobResult":
        if not error_code:
            raise ValueError("error_code must not be empty")
        return cls(
            processed_count=0,
            skipped_count=0,
            error_code=error_code,
            error_message=error_message,
        )


class JobRepository(Protocol):
    def claim_next(
        self,
        *,
        worker_id: str,
        claimed_at: datetime,
        lease_duration: timedelta,
    ) -> ClaimedJob | None: ...

    def heartbeat(
        self,
        *,
        job_id: UUID,
        lease: JobLease,
        heartbeat_at: datetime,
        lease_duration: timedelta,
    ) -> JobLease: ...

    def mark_succeeded(
        self,
        *,
        job_id: UUID,
        lease: JobLease,
        finished_at: datetime,
        processed_count: int,
        skipped_count: int,
    ) -> None: ...

    def mark_failed(
        self,
        *,
        job_id: UUID,
        lease: JobLease,
        finished_at: datetime,
        error_code: str,
        error_message: str | None,
    ) -> None: ...


class JobRunner(Protocol):
    def run(self, job: ClaimedJob, cancellation: Event) -> JobResult: ...


class ChatReplayJobRunner:
    def __init__(
        self,
        *,
        collector: ChatReplayCollector,
        clock: Callable[[], datetime],
        attempt_timeout: timedelta,
    ) -> None:
        if attempt_timeout <= timedelta(0):
            raise ValueError("attempt_timeout must be positive")
        self._collector = collector
        self._clock = clock
        self._attempt_timeout = attempt_timeout

    def run(self, job: ClaimedJob, cancellation: Event) -> JobResult:
        try:
            result = self._collector.collect(
                CollectionRequest(
                    collection_job_id=job.id,
                    stream_id=job.stream_id,
                    canonical_youtube_url=job.canonical_youtube_url,
                    attempt=job.attempt,
                    deadline=self._clock() + self._attempt_timeout,
                ),
                cancellation,
            )
        except CollectionFailure as failure:
            return JobResult.failed(
                error_code=failure.code.value,
                error_message=failure.safe_message,
            )
        return JobResult.succeeded(
            processed_count=result.saved_message_count,
            skipped_count=result.skipped_action_count,
        )


class ClaimLoop:
    def __init__(
        self,
        *,
        repository: JobRepository,
        runner: JobRunner,
        worker_id: str,
        lease_duration: timedelta,
        clock: Callable[[], datetime],
        heartbeat_interval: timedelta = timedelta(seconds=30),
        poll_interval: timedelta = timedelta(seconds=1),
        metric_sink: MetricSink | None = None,
    ) -> None:
        if not worker_id:
            raise ValueError("worker_id must not be empty")
        if lease_duration <= timedelta(0):
            raise ValueError("lease_duration must be positive")
        if heartbeat_interval <= timedelta(0) or heartbeat_interval >= lease_duration:
            raise ValueError("heartbeat_interval must be positive and shorter than lease_duration")
        if poll_interval <= timedelta(0):
            raise ValueError("poll_interval must be positive")
        self._repository = repository
        self._runner = runner
        self._worker_id = worker_id
        self._lease_duration = lease_duration
        self._clock = clock
        self._heartbeat_interval = heartbeat_interval
        self._poll_interval = poll_interval
        self._metric_sink = metric_sink or NullMetricSink()

    def run_until_cancelled(self, cancellation: Event) -> None:
        while not cancellation.is_set():
            did_work = self.run_once(cancellation)
            if not did_work:
                cancellation.wait(self._poll_interval.total_seconds())

    def run_once(self, cancellation: Event) -> bool:
        if cancellation.is_set():
            return False

        job = self._repository.claim_next(
            worker_id=self._worker_id,
            claimed_at=self._clock(),
            lease_duration=self._lease_duration,
        )
        if job is None:
            return False
        started_at = monotonic()

        lease = [job.lease]
        lease_lock = Lock()
        heartbeat_stop = Event()
        heartbeat_error: list[Exception] = []

        def heartbeat_until_stopped() -> None:
            while not heartbeat_stop.wait(self._heartbeat_interval.total_seconds()):
                try:
                    with lease_lock:
                        lease[0] = self._repository.heartbeat(
                            job_id=job.id,
                            lease=lease[0],
                            heartbeat_at=self._clock(),
                            lease_duration=self._lease_duration,
                        )
                except Exception as error:
                    heartbeat_error.append(error)
                    cancellation.set()
                    return

        heartbeat_thread = Thread(target=heartbeat_until_stopped, daemon=True)
        heartbeat_thread.start()
        try:
            result = self._runner.run(job, cancellation)
        finally:
            heartbeat_stop.set()
            heartbeat_thread.join()

        if heartbeat_error:
            raise heartbeat_error[0]

        finished_at = self._clock()
        with lease_lock:
            final_lease = lease[0]
        if result.error_code is None:
            self._repository.mark_succeeded(
                job_id=job.id,
                lease=final_lease,
                finished_at=finished_at,
                processed_count=result.processed_count,
                skipped_count=result.skipped_count,
            )
            outcome = "succeeded"
        else:
            error_code = safe_error_code(result.error_code)
            self._repository.mark_failed(
                job_id=job.id,
                lease=final_lease,
                finished_at=finished_at,
                error_code=error_code,
                error_message="Collection job failed.",
            )
            outcome = "failed"
        self._metric_sink.emit(
            JobMetric(
                job_id=str(job.id),
                job_kind=job.kind,
                attempt=job.attempt,
                outcome=outcome,
                duration_seconds=monotonic() - started_at,
                processed_count=result.processed_count,
                skipped_count=result.skipped_count,
                error_code=(
                    safe_error_code(result.error_code) if result.error_code is not None else None
                ),
            )
        )
        return True
