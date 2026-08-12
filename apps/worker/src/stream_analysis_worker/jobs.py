from dataclasses import dataclass
from datetime import datetime, timedelta
from typing import Any
from uuid import UUID

from psycopg_pool import ConnectionPool


class LeaseLostError(RuntimeError):
    pass


@dataclass(frozen=True, slots=True)
class JobLease:
    worker_id: str
    token: UUID
    heartbeat_at: datetime
    expires_at: datetime


@dataclass(frozen=True, slots=True)
class ClaimedJob:
    id: UUID
    stream_id: UUID
    kind: str
    attempt: int
    started_at: datetime
    lease: JobLease


class PostgresJobRepository:
    def __init__(self, pool: ConnectionPool[Any]) -> None:
        self._pool = pool

    def claim_next(
        self,
        *,
        worker_id: str,
        claimed_at: datetime,
        lease_duration: timedelta,
    ) -> ClaimedJob | None:
        if not worker_id:
            raise ValueError("worker_id must not be empty")
        if lease_duration <= timedelta(0):
            raise ValueError("lease_duration must be positive")

        with self._pool.connection() as connection:
            row = connection.execute(
                """
                WITH candidate AS (
                    SELECT id
                    FROM collection.collection_jobs
                    WHERE status = 'queued'
                       OR (status = 'running' AND lease_expires_at <= %(claimed_at)s)
                    ORDER BY requested_at, id
                    FOR UPDATE SKIP LOCKED
                    LIMIT 1
                )
                UPDATE collection.collection_jobs AS job
                SET status = 'running',
                    attempt = job.attempt + 1,
                    worker_id = %(worker_id)s,
                    lease_token = gen_random_uuid(),
                    heartbeat_at = %(claimed_at)s,
                    lease_expires_at = %(claimed_at)s + %(lease_duration)s,
                    started_at = COALESCE(job.started_at, %(claimed_at)s),
                    finished_at = NULL,
                    error_code = NULL,
                    error_message = NULL,
                    updated_at = %(claimed_at)s
                FROM candidate
                WHERE job.id = candidate.id
                RETURNING
                    job.id,
                    job.stream_id,
                    job.kind,
                    job.attempt,
                    job.started_at,
                    job.worker_id,
                    job.lease_token,
                    job.heartbeat_at,
                    job.lease_expires_at
                """,
                {
                    "worker_id": worker_id,
                    "claimed_at": claimed_at,
                    "lease_duration": lease_duration,
                },
            ).fetchone()

        if row is None:
            return None
        return claimed_job_from_row(row)

    def heartbeat(
        self,
        *,
        job_id: UUID,
        lease: JobLease,
        heartbeat_at: datetime,
        lease_duration: timedelta,
    ) -> JobLease:
        if lease_duration <= timedelta(0):
            raise ValueError("lease_duration must be positive")

        with self._pool.connection() as connection:
            row = connection.execute(
                """
                UPDATE collection.collection_jobs
                SET heartbeat_at = %(heartbeat_at)s,
                    lease_expires_at = %(heartbeat_at)s + %(lease_duration)s,
                    updated_at = %(heartbeat_at)s
                WHERE id = %(job_id)s
                  AND status = 'running'
                  AND worker_id = %(worker_id)s
                  AND lease_token = %(lease_token)s
                  AND heartbeat_at <= %(heartbeat_at)s
                  AND lease_expires_at > %(heartbeat_at)s
                RETURNING worker_id, lease_token, heartbeat_at, lease_expires_at
                """,
                {
                    "job_id": job_id,
                    "worker_id": lease.worker_id,
                    "lease_token": lease.token,
                    "heartbeat_at": heartbeat_at,
                    "lease_duration": lease_duration,
                },
            ).fetchone()

        if row is None:
            raise LeaseLostError(f"job {job_id} lease is no longer owned by this worker")
        return lease_from_row(row)

    def mark_succeeded(
        self,
        *,
        job_id: UUID,
        lease: JobLease,
        finished_at: datetime,
        processed_count: int,
        skipped_count: int,
    ) -> None:
        if processed_count < 0 or skipped_count < 0:
            raise ValueError("job counts must not be negative")

        with self._pool.connection() as connection:
            row = connection.execute(
                """
                UPDATE collection.collection_jobs
                SET status = 'succeeded',
                    worker_id = NULL,
                    lease_token = NULL,
                    lease_expires_at = NULL,
                    processed_count = %(processed_count)s,
                    skipped_count = %(skipped_count)s,
                    finished_at = %(finished_at)s,
                    error_code = NULL,
                    error_message = NULL,
                    updated_at = %(finished_at)s
                WHERE id = %(job_id)s
                  AND status = 'running'
                  AND worker_id = %(worker_id)s
                  AND lease_token = %(lease_token)s
                  AND heartbeat_at <= %(finished_at)s
                  AND lease_expires_at > %(finished_at)s
                RETURNING id
                """,
                {
                    "job_id": job_id,
                    "worker_id": lease.worker_id,
                    "lease_token": lease.token,
                    "finished_at": finished_at,
                    "processed_count": processed_count,
                    "skipped_count": skipped_count,
                },
            ).fetchone()

        if row is None:
            raise LeaseLostError(f"job {job_id} lease is no longer owned by this worker")

    def record_progress(
        self,
        *,
        job_id: UUID,
        lease: JobLease,
        recorded_at: datetime,
        processed_count: int,
        skipped_count: int,
    ) -> None:
        if processed_count < 0 or skipped_count < 0:
            raise ValueError("job counts must not be negative")

        with self._pool.connection() as connection:
            row = connection.execute(
                """
                UPDATE collection.collection_jobs
                SET processed_count = %(processed_count)s,
                    skipped_count = %(skipped_count)s,
                    updated_at = %(recorded_at)s
                WHERE id = %(job_id)s
                  AND status = 'running'
                  AND worker_id = %(worker_id)s
                  AND lease_token = %(lease_token)s
                  AND heartbeat_at <= %(recorded_at)s
                  AND lease_expires_at > %(recorded_at)s
                  AND processed_count <= %(processed_count)s
                  AND skipped_count <= %(skipped_count)s
                RETURNING id
                """,
                {
                    "job_id": job_id,
                    "worker_id": lease.worker_id,
                    "lease_token": lease.token,
                    "recorded_at": recorded_at,
                    "processed_count": processed_count,
                    "skipped_count": skipped_count,
                },
            ).fetchone()

        if row is None:
            raise LeaseLostError(f"job {job_id} lease is no longer owned by this worker")

    def mark_failed(
        self,
        *,
        job_id: UUID,
        lease: JobLease,
        finished_at: datetime,
        error_code: str,
        error_message: str | None,
    ) -> None:
        if not error_code:
            raise ValueError("error_code must not be empty")

        with self._pool.connection() as connection:
            row = connection.execute(
                """
                UPDATE collection.collection_jobs
                SET status = 'failed',
                    worker_id = NULL,
                    lease_token = NULL,
                    lease_expires_at = NULL,
                    finished_at = %(finished_at)s,
                    error_code = %(error_code)s,
                    error_message = %(error_message)s,
                    updated_at = %(finished_at)s
                WHERE id = %(job_id)s
                  AND status = 'running'
                  AND worker_id = %(worker_id)s
                  AND lease_token = %(lease_token)s
                  AND heartbeat_at <= %(finished_at)s
                  AND lease_expires_at > %(finished_at)s
                RETURNING id
                """,
                {
                    "job_id": job_id,
                    "worker_id": lease.worker_id,
                    "lease_token": lease.token,
                    "finished_at": finished_at,
                    "error_code": error_code,
                    "error_message": error_message,
                },
            ).fetchone()

        if row is None:
            raise LeaseLostError(f"job {job_id} lease is no longer owned by this worker")


def claimed_job_from_row(row: tuple[Any, ...]) -> ClaimedJob:
    (
        job_id,
        stream_id,
        kind,
        attempt,
        started_at,
        worker_id,
        lease_token,
        heartbeat_at,
        lease_expires_at,
    ) = row
    return ClaimedJob(
        id=job_id,
        stream_id=stream_id,
        kind=kind,
        attempt=attempt,
        started_at=started_at,
        lease=lease_from_row((worker_id, lease_token, heartbeat_at, lease_expires_at)),
    )


def lease_from_row(row: tuple[Any, ...]) -> JobLease:
    worker_id, lease_token, heartbeat_at, lease_expires_at = row
    return JobLease(
        worker_id=worker_id,
        token=lease_token,
        heartbeat_at=heartbeat_at,
        expires_at=lease_expires_at,
    )
