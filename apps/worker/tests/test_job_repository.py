import os
from collections.abc import Iterator
from concurrent.futures import ThreadPoolExecutor
from datetime import UTC, datetime, timedelta
from pathlib import Path
from threading import Event, Lock
from time import sleep
from uuid import UUID

os.environ.setdefault("TESTCONTAINERS_RYUK_DISABLED", "true")

import psycopg
import pytest
from psycopg_pool import ConnectionPool
from testcontainers.community.postgres import PostgresContainer

from stream_analysis_worker.jobs import ClaimedJob, LeaseLostError, PostgresJobRepository
from stream_analysis_worker.worker import ClaimLoop, JobResult

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
        connection.execute("TRUNCATE collection.collection_jobs, stream.streams CASCADE")


def test_only_one_worker_claims_a_queued_job(database_url: str) -> None:
    job_id = seed_collection_job(database_url, "exclusive-claim")
    claimed_at = datetime(2026, 8, 12, 1, 0, tzinfo=UTC)

    with (
        ConnectionPool(database_url, min_size=1, max_size=1) as first_pool,
        ConnectionPool(database_url, min_size=1, max_size=1) as second_pool,
    ):
        repositories = [PostgresJobRepository(first_pool), PostgresJobRepository(second_pool)]
        with ThreadPoolExecutor(max_workers=2) as executor:
            claims = list(
                executor.map(
                    lambda item: item[0].claim_next(
                        worker_id=item[1],
                        claimed_at=claimed_at,
                        lease_duration=timedelta(minutes=2),
                    ),
                    zip(repositories, ["worker-a", "worker-b"], strict=True),
                )
            )

    claimed_jobs = [claim for claim in claims if claim is not None]
    assert len(claimed_jobs) == 1
    assert claimed_jobs[0].id == job_id
    assert claimed_jobs[0].attempt == 1
    assert claimed_jobs[0].lease.worker_id in {"worker-a", "worker-b"}
    assert claimed_jobs[0].lease.expires_at == claimed_at + timedelta(minutes=2)


def test_collection_steps_accept_only_the_initial_workflow(database_url: str) -> None:
    job_id = seed_collection_job(database_url, "collection-steps")

    with psycopg.connect(database_url) as connection:
        connection.execute(
            """
            INSERT INTO collection.collection_steps (job_id, name)
            VALUES (%s, 'metadata'), (%s, 'chat_replay')
            """,
            (job_id, job_id),
        )

    with (
        pytest.raises(psycopg.errors.CheckViolation),
        psycopg.connect(database_url) as connection,
    ):
        connection.execute(
            """
            INSERT INTO collection.collection_steps (job_id, name)
            VALUES (%s, 'media')
            """,
            (job_id,),
        )

    with psycopg.connect(database_url) as connection:
        names = connection.execute(
            """
            SELECT name
            FROM collection.collection_steps
            WHERE job_id = %s
            ORDER BY name
            """,
            (job_id,),
        ).fetchall()

    assert names == [("chat_replay",), ("metadata",)]


def test_current_lease_owner_can_extend_the_heartbeat(database_url: str) -> None:
    seed_collection_job(database_url, "heartbeat-owner")
    claimed_at = datetime(2026, 8, 12, 2, 0, tzinfo=UTC)
    heartbeat_at = claimed_at + timedelta(seconds=30)

    with ConnectionPool(database_url, min_size=1, max_size=1) as pool:
        repository = PostgresJobRepository(pool)
        job = repository.claim_next(
            worker_id="worker-heartbeat",
            claimed_at=claimed_at,
            lease_duration=timedelta(minutes=2),
        )
        assert job is not None

        renewed = repository.heartbeat(
            job_id=job.id,
            lease=job.lease,
            heartbeat_at=heartbeat_at,
            lease_duration=timedelta(minutes=2),
        )

    assert renewed.token == job.lease.token
    assert renewed.worker_id == job.lease.worker_id
    assert renewed.heartbeat_at == heartbeat_at
    assert renewed.expires_at == heartbeat_at + timedelta(minutes=2)


def test_expired_job_is_reclaimed_and_the_old_lease_is_rejected(database_url: str) -> None:
    job_id = seed_collection_job(database_url, "lease-recovery")
    first_claimed_at = datetime(2026, 8, 12, 3, 0, tzinfo=UTC)
    recovered_at = first_claimed_at + timedelta(minutes=2, seconds=1)

    with ConnectionPool(database_url, min_size=1, max_size=2) as pool:
        repository = PostgresJobRepository(pool)
        first = repository.claim_next(
            worker_id="worker-before-restart",
            claimed_at=first_claimed_at,
            lease_duration=timedelta(minutes=2),
        )
        assert first is not None

        recovered = repository.claim_next(
            worker_id="worker-after-restart",
            claimed_at=recovered_at,
            lease_duration=timedelta(minutes=2),
        )
        assert recovered is not None

        with pytest.raises(LeaseLostError):
            repository.heartbeat(
                job_id=first.id,
                lease=first.lease,
                heartbeat_at=recovered_at + timedelta(seconds=1),
                lease_duration=timedelta(minutes=2),
            )
        with pytest.raises(LeaseLostError):
            repository.record_progress(
                job_id=first.id,
                lease=first.lease,
                recorded_at=recovered_at + timedelta(seconds=1),
                processed_count=1,
                skipped_count=0,
            )

    assert recovered.id == job_id
    assert recovered.attempt == 2
    assert recovered.lease.worker_id == "worker-after-restart"
    assert recovered.lease.token != first.lease.token


def test_current_lease_owner_can_succeed_a_job_once(database_url: str) -> None:
    seed_collection_job(database_url, "successful-job")
    claimed_at = datetime(2026, 8, 12, 4, 0, tzinfo=UTC)

    with ConnectionPool(database_url, min_size=1, max_size=1) as pool:
        repository = PostgresJobRepository(pool)
        job = repository.claim_next(
            worker_id="worker-success",
            claimed_at=claimed_at,
            lease_duration=timedelta(minutes=2),
        )
        assert job is not None

        repository.mark_succeeded(
            job_id=job.id,
            lease=job.lease,
            finished_at=claimed_at + timedelta(minutes=1),
            processed_count=120,
            skipped_count=3,
        )

        next_job = repository.claim_next(
            worker_id="worker-later",
            claimed_at=claimed_at + timedelta(hours=1),
            lease_duration=timedelta(minutes=2),
        )

    assert next_job is None


def test_current_lease_owner_can_record_monotonic_progress(database_url: str) -> None:
    job_id = seed_collection_job(database_url, "job-progress")
    claimed_at = datetime(2026, 8, 12, 4, 30, tzinfo=UTC)

    with ConnectionPool(database_url, min_size=1, max_size=1) as pool:
        repository = PostgresJobRepository(pool)
        job = repository.claim_next(
            worker_id="worker-progress",
            claimed_at=claimed_at,
            lease_duration=timedelta(minutes=2),
        )
        assert job is not None

        repository.record_progress(
            job_id=job.id,
            lease=job.lease,
            recorded_at=claimed_at + timedelta(seconds=30),
            processed_count=75,
            skipped_count=4,
        )

    with psycopg.connect(database_url) as connection:
        counts = connection.execute(
            """
            SELECT processed_count, skipped_count
            FROM collection.collection_jobs
            WHERE id = %s
            """,
            (job_id,),
        ).fetchone()

    assert counts == (75, 4)


def test_failed_job_stays_terminal_and_retry_uses_a_new_job(database_url: str) -> None:
    failed_job_id = seed_collection_job(database_url, "safe-retry")
    claimed_at = datetime(2026, 8, 12, 5, 0, tzinfo=UTC)

    with ConnectionPool(database_url, min_size=1, max_size=1) as pool:
        repository = PostgresJobRepository(pool)
        failed_job = repository.claim_next(
            worker_id="worker-failure",
            claimed_at=claimed_at,
            lease_duration=timedelta(minutes=2),
        )
        assert failed_job is not None

        repository.mark_failed(
            job_id=failed_job.id,
            lease=failed_job.lease,
            finished_at=claimed_at + timedelta(minutes=1),
            error_code="CHAT_REPLAY_UNAVAILABLE",
            error_message="Chat replay is unavailable.",
        )

        assert (
            repository.claim_next(
                worker_id="worker-must-not-reuse-failure",
                claimed_at=claimed_at + timedelta(hours=1),
                lease_duration=timedelta(minutes=2),
            )
            is None
        )

        retry_job_id = seed_retry_job(database_url, "safe-retry")
        retry = repository.claim_next(
            worker_id="worker-retry",
            claimed_at=claimed_at + timedelta(hours=1),
            lease_duration=timedelta(minutes=2),
        )

    assert retry is not None
    assert failed_job_id != retry_job_id
    assert retry.id == retry_job_id
    assert retry.attempt == 1


def test_claim_loop_runs_and_succeeds_a_claimed_job(database_url: str) -> None:
    job_id = seed_collection_job(database_url, "claim-loop-success")
    claimed_at = datetime(2026, 8, 12, 6, 0, tzinfo=UTC)
    runner = SuccessfulRunner()

    with ConnectionPool(database_url, min_size=1, max_size=1) as pool:
        repository = PostgresJobRepository(pool)
        loop = ClaimLoop(
            repository=repository,
            runner=runner,
            worker_id="worker-loop",
            lease_duration=timedelta(minutes=2),
            clock=lambda: claimed_at,
        )

        did_work = loop.run_once(Event())
        next_job = repository.claim_next(
            worker_id="worker-later",
            claimed_at=claimed_at + timedelta(hours=1),
            lease_duration=timedelta(minutes=2),
        )

    assert did_work is True
    assert [job.id for job in runner.jobs] == [job_id]
    assert next_job is None


def test_claim_loop_heartbeats_while_the_runner_is_active(database_url: str) -> None:
    job_id = seed_collection_job(database_url, "claim-loop-heartbeat")
    claimed_at = datetime(2026, 8, 12, 7, 0, tzinfo=UTC)
    clock = ControlledClock(claimed_at)
    runner = BlockingRunner()

    with ConnectionPool(database_url, min_size=1, max_size=3) as pool:
        repository = PostgresJobRepository(pool)
        loop = ClaimLoop(
            repository=repository,
            runner=runner,
            worker_id="worker-heartbeat-loop",
            lease_duration=timedelta(minutes=2),
            heartbeat_interval=timedelta(milliseconds=50),
            clock=clock,
        )

        with ThreadPoolExecutor(max_workers=1) as executor:
            running = executor.submit(loop.run_once, Event())
            assert runner.started.wait(timeout=2)
            clock.advance(timedelta(minutes=1))
            wait_for_heartbeat(database_url, job_id, claimed_at + timedelta(minutes=1))

            competing_claim = repository.claim_next(
                worker_id="worker-competing",
                claimed_at=claimed_at + timedelta(minutes=2, seconds=1),
                lease_duration=timedelta(minutes=2),
            )
            runner.release.set()
            assert running.result(timeout=2) is True

    assert competing_claim is None


def test_claim_loop_stops_after_finishing_claimed_work(database_url: str) -> None:
    job_id = seed_collection_job(database_url, "claim-loop-stop")
    claimed_at = datetime(2026, 8, 12, 8, 0, tzinfo=UTC)
    cancellation = Event()
    runner = StopAfterSuccessRunner()

    with ConnectionPool(database_url, min_size=1, max_size=1) as pool:
        repository = PostgresJobRepository(pool)
        loop = ClaimLoop(
            repository=repository,
            runner=runner,
            worker_id="worker-stop-loop",
            lease_duration=timedelta(minutes=2),
            clock=lambda: claimed_at,
        )

        loop.run_until_cancelled(cancellation)

    assert cancellation.is_set() is True
    assert [job.id for job in runner.jobs] == [job_id]


class SuccessfulRunner:
    def __init__(self) -> None:
        self.jobs: list[ClaimedJob] = []

    def run(self, job: ClaimedJob, cancellation: Event) -> JobResult:
        self.jobs.append(job)
        assert cancellation.is_set() is False
        return JobResult.succeeded(processed_count=50, skipped_count=2)


class BlockingRunner:
    def __init__(self) -> None:
        self.started = Event()
        self.release = Event()

    def run(self, job: ClaimedJob, cancellation: Event) -> JobResult:
        self.started.set()
        assert self.release.wait(timeout=2)
        assert cancellation.is_set() is False
        return JobResult.succeeded(processed_count=10, skipped_count=0)


class StopAfterSuccessRunner:
    def __init__(self) -> None:
        self.jobs: list[ClaimedJob] = []

    def run(self, job: ClaimedJob, cancellation: Event) -> JobResult:
        self.jobs.append(job)
        cancellation.set()
        return JobResult.succeeded(processed_count=1, skipped_count=0)


class ControlledClock:
    def __init__(self, current: datetime) -> None:
        self._current = current
        self._lock = Lock()

    def __call__(self) -> datetime:
        with self._lock:
            return self._current

    def advance(self, duration: timedelta) -> None:
        with self._lock:
            self._current += duration


def seed_collection_job(database_url: str, video_id: str) -> UUID:
    with psycopg.connect(database_url) as connection:
        stream_id = connection.execute(
            """
            INSERT INTO stream.streams (
                youtube_video_id,
                canonical_url,
                title,
                channel_id,
                channel_title,
                lifecycle_status,
                metadata_fetched_at
            ) VALUES (%s, %s, %s, %s, %s, 'ended', CURRENT_TIMESTAMP)
            RETURNING id
            """,
            (
                video_id,
                f"https://www.youtube.com/watch?v={video_id}",
                "Fixture stream",
                "fixture-channel",
                "Fixture channel",
            ),
        ).fetchone()
        assert stream_id is not None

        job_id = connection.execute(
            """
            INSERT INTO collection.collection_jobs (stream_id, kind)
            VALUES (%s, 'chat_replay')
            RETURNING id
            """,
            (stream_id[0],),
        ).fetchone()
        assert job_id is not None
        value = job_id[0]
        assert isinstance(value, UUID)
        return value


def seed_retry_job(database_url: str, video_id: str) -> UUID:
    with psycopg.connect(database_url) as connection:
        job_id = connection.execute(
            """
            INSERT INTO collection.collection_jobs (stream_id, kind)
            SELECT id, 'chat_replay'
            FROM stream.streams
            WHERE youtube_video_id = %s
            RETURNING id
            """,
            (video_id,),
        ).fetchone()
        assert job_id is not None
        value = job_id[0]
        assert isinstance(value, UUID)
        return value


def migrations_directory() -> Path:
    return Path(__file__).resolve().parents[3] / "migrations"


def wait_for_heartbeat(
    database_url: str,
    job_id: UUID,
    expected_heartbeat: datetime,
) -> None:
    for _ in range(200):
        with psycopg.connect(database_url) as connection:
            heartbeat = connection.execute(
                """
                SELECT heartbeat_at
                FROM collection.collection_jobs
                WHERE id = %s
                """,
                (job_id,),
            ).fetchone()
        if heartbeat == (expected_heartbeat,):
            return
        sleep(0.01)
    pytest.fail(f"heartbeat did not reach {expected_heartbeat.isoformat()}")
