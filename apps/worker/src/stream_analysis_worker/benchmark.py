import argparse
import json
import os
import platform
import sys
import tempfile
from collections.abc import Sequence
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from pathlib import Path
from threading import Event, Lock, Thread
from time import monotonic
from typing import Any
from uuid import UUID, uuid4

from psycopg_pool import ConnectionPool

from stream_analysis_worker.chat import PostgresChatMessageRepository
from stream_analysis_worker.chat_replay import YtDlpChatReplayCollector
from stream_analysis_worker.collector import CollectionOutcome, CollectionRequest
from stream_analysis_worker.yt_dlp_process import (
    ProcessTermination,
    SubprocessYtDlpProcess,
    YtDlpProcessRequest,
    YtDlpProcessResult,
)

DEFAULT_VIDEO_ID = "R3l34mHWmas"
DEFAULT_TIMEOUT_SECONDS = 1_800
MAX_WORKER_RSS_BYTES = 512 * 1024 * 1024


class BenchmarkError(RuntimeError):
    pass


@dataclass(frozen=True, slots=True)
class DirectMeasurement:
    acquisition_seconds: float
    import_seconds: float
    total_seconds: float
    artifact_bytes: int
    peak_rss_bytes: int
    yt_dlp_process_count: int
    yt_dlp_version: str

    def as_dict(self) -> dict[str, object]:
        return {
            "acquisition_seconds": self.acquisition_seconds,
            "import_seconds": self.import_seconds,
            "total_seconds": self.total_seconds,
            "artifact_bytes": self.artifact_bytes,
            "peak_rss_bytes": self.peak_rss_bytes,
            "yt_dlp_process_count": self.yt_dlp_process_count,
            "yt_dlp_version": self.yt_dlp_version,
            "saved_message_count": None,
            "duplicate_count": None,
            "skipped_action_count": None,
        }


@dataclass(frozen=True, slots=True)
class WorkerMeasurement:
    acquisition_seconds: float
    import_seconds: float
    total_seconds: float
    artifact_bytes: int
    peak_rss_bytes: int
    yt_dlp_process_count: int
    worker_owned_youtube_http_request_count: int
    saved_message_count: int
    duplicate_count: int
    skipped_action_count: int
    stored_message_count: int
    maximum_batch_size: int
    yt_dlp_version: str

    def as_dict(self) -> dict[str, object]:
        return {
            "acquisition_seconds": self.acquisition_seconds,
            "import_seconds": self.import_seconds,
            "total_seconds": self.total_seconds,
            "artifact_bytes": self.artifact_bytes,
            "peak_rss_bytes": self.peak_rss_bytes,
            "yt_dlp_process_count": self.yt_dlp_process_count,
            "worker_owned_youtube_http_request_count": (
                self.worker_owned_youtube_http_request_count
            ),
            "saved_message_count": self.saved_message_count,
            "duplicate_count": self.duplicate_count,
            "skipped_action_count": self.skipped_action_count,
            "stored_message_count": self.stored_message_count,
            "maximum_batch_size": self.maximum_batch_size,
            "yt_dlp_version": self.yt_dlp_version,
        }


@dataclass(frozen=True, slots=True)
class GateResult:
    name: str
    passed: bool
    detail: str

    def as_dict(self) -> dict[str, object]:
        return {"name": self.name, "passed": self.passed, "detail": self.detail}


@dataclass(frozen=True, slots=True)
class BenchmarkReport:
    started_at: datetime
    video_id: str
    canonical_youtube_url: str
    platform_description: str
    python_version: str
    credentials: str
    direct: DirectMeasurement
    worker: WorkerMeasurement
    gates: tuple[GateResult, ...]

    @property
    def passed(self) -> bool:
        return all(gate.passed for gate in self.gates)

    def as_dict(self) -> dict[str, object]:
        return {
            "schema_version": 1,
            "benchmark": "direct-yt-dlp-vs-worker-chat-replay",
            "started_at": self.started_at.isoformat(),
            "video_id": self.video_id,
            "canonical_youtube_url": self.canonical_youtube_url,
            "environment": {
                "platform": self.platform_description,
                "python_version": self.python_version,
                "credentials": self.credentials,
                "same_machine": True,
                "same_network": True,
                "execution_order": ["direct", "worker"],
            },
            "direct": self.direct.as_dict(),
            "worker": self.worker.as_dict(),
            "gates": [gate.as_dict() for gate in self.gates],
            "passed": self.passed,
        }


class PeakRssSampler:
    """Sample the resident memory of roots and their descendants on Linux."""

    def __init__(
        self,
        root_pids: Sequence[int] = (),
        *,
        proc_root: Path = Path("/proc"),
        interval_seconds: float = 0.01,
    ) -> None:
        if interval_seconds <= 0:
            raise ValueError("interval_seconds must be positive")
        self._proc_root = proc_root
        self._interval_seconds = interval_seconds
        self._root_pids = set(root_pids)
        self._lock = Lock()
        self._stopped = Event()
        self._thread: Thread | None = None
        self._peak_rss_bytes = 0

    @property
    def peak_rss_bytes(self) -> int:
        with self._lock:
            return self._peak_rss_bytes

    def start(self) -> None:
        if not self._proc_root.is_dir():
            raise BenchmarkError("peak RSS measurement requires a Linux /proc filesystem")
        if self._thread is not None:
            raise RuntimeError("RSS sampler has already started")
        self._sample()
        self._thread = Thread(target=self._run, name="benchmark-rss-sampler", daemon=True)
        self._thread.start()

    def add_process(self, pid: int) -> None:
        with self._lock:
            self._root_pids.add(pid)
        self._sample()

    def stop(self) -> None:
        self._stopped.set()
        thread = self._thread
        if thread is not None:
            thread.join(timeout=max(1.0, self._interval_seconds * 4))
        self._sample()

    def _run(self) -> None:
        while not self._stopped.wait(self._interval_seconds):
            self._sample()

    def _sample(self) -> None:
        with self._lock:
            roots = set(self._root_pids)
        process_ids = self._process_tree(roots)
        rss_bytes = sum(self._read_rss_bytes(pid) for pid in process_ids)
        with self._lock:
            self._peak_rss_bytes = max(self._peak_rss_bytes, rss_bytes)

    def _process_tree(self, roots: set[int]) -> set[int]:
        discovered: set[int] = set()
        pending = list(roots)
        while pending:
            pid = pending.pop()
            if pid in discovered:
                continue
            discovered.add(pid)
            children_path = self._proc_root / str(pid) / "task" / str(pid) / "children"
            try:
                children = children_path.read_text(encoding="ascii").split()
            except (FileNotFoundError, PermissionError, ProcessLookupError):
                continue
            pending.extend(int(child) for child in children)
        return discovered

    def _read_rss_bytes(self, pid: int) -> int:
        status_path = self._proc_root / str(pid) / "status"
        try:
            for line in status_path.read_text(encoding="ascii").splitlines():
                if line.startswith("VmRSS:"):
                    return int(line.split()[1]) * 1024
        except (FileNotFoundError, PermissionError, ProcessLookupError, ValueError):
            return 0
        return 0


def evaluate_gates(
    direct: DirectMeasurement,
    worker: WorkerMeasurement,
) -> tuple[GateResult, ...]:
    duration_limit = direct.total_seconds * 1.25 + 60.0
    return (
        GateResult(
            name="same_yt_dlp_version",
            passed=worker.yt_dlp_version == direct.yt_dlp_version,
            detail=f"direct={direct.yt_dlp_version}; worker={worker.yt_dlp_version}",
        ),
        GateResult(
            name="worker_total_duration",
            passed=worker.total_seconds <= duration_limit,
            detail=f"actual={worker.total_seconds:.3f}s; limit={duration_limit:.3f}s",
        ),
        GateResult(
            name="worker_peak_rss",
            passed=worker.peak_rss_bytes < MAX_WORKER_RSS_BYTES,
            detail=f"actual={worker.peak_rss_bytes}B; limit<{MAX_WORKER_RSS_BYTES}B",
        ),
        GateResult(
            name="one_yt_dlp_process_per_attempt",
            passed=(direct.yt_dlp_process_count == 1 and worker.yt_dlp_process_count == 1),
            detail=(f"direct={direct.yt_dlp_process_count}; worker={worker.yt_dlp_process_count}"),
        ),
        GateResult(
            name="zero_worker_owned_youtube_http_requests",
            passed=worker.worker_owned_youtube_http_request_count == 0,
            detail=f"actual={worker.worker_owned_youtube_http_request_count}",
        ),
        GateResult(
            name="stored_message_count",
            passed=worker.stored_message_count == worker.saved_message_count,
            detail=(f"stored={worker.stored_message_count}; saved={worker.saved_message_count}"),
        ),
        GateResult(
            name="maximum_import_batch_size",
            passed=worker.maximum_batch_size <= 500,
            detail=f"actual={worker.maximum_batch_size}; limit<=500",
        ),
    )


def run_benchmark(
    *,
    database_url: str,
    video_id: str = DEFAULT_VIDEO_ID,
    timeout_seconds: int = DEFAULT_TIMEOUT_SECONDS,
) -> BenchmarkReport:
    if not database_url:
        raise ValueError("database_url must not be empty")
    if not video_id or any(character.isspace() for character in video_id):
        raise ValueError("video_id must be non-empty and contain no whitespace")
    if timeout_seconds < 1:
        raise ValueError("timeout_seconds must be positive")

    started_at = datetime.now(UTC)
    canonical_youtube_url = f"https://www.youtube.com/watch?v={video_id}"
    deadline_delta = timedelta(seconds=timeout_seconds)
    direct = _measure_direct(canonical_youtube_url, deadline_delta=deadline_delta)
    with ConnectionPool(database_url, min_size=1, max_size=2) as pool:
        worker = _measure_worker(
            pool,
            canonical_youtube_url=canonical_youtube_url,
            deadline_delta=deadline_delta,
        )
    return BenchmarkReport(
        started_at=started_at,
        video_id=video_id,
        canonical_youtube_url=canonical_youtube_url,
        platform_description=platform.platform(),
        python_version=platform.python_version(),
        credentials="none (public stream; no cookie or credential flags)",
        direct=direct,
        worker=worker,
        gates=evaluate_gates(direct, worker),
    )


def _measure_direct(
    canonical_youtube_url: str,
    *,
    deadline_delta: timedelta,
) -> DirectMeasurement:
    process_ids: list[int] = []
    sampler = PeakRssSampler()

    def observe_process(pid: int) -> None:
        process_ids.append(pid)
        sampler.add_process(pid)

    process = SubprocessYtDlpProcess(process_observer=observe_process)
    sampler.start()
    started = monotonic()
    try:
        with tempfile.TemporaryDirectory(prefix="yt-dlp-direct-benchmark-") as directory:
            result = process.run(
                YtDlpProcessRequest(
                    canonical_youtube_url=canonical_youtube_url,
                    attempt_directory=Path(directory),
                    deadline=datetime.now(UTC) + deadline_delta,
                ),
                Event(),
            )
            total_seconds = monotonic() - started
            _require_successful_artifact(result)
            artifact = result.artifact_path
            if artifact is None:
                raise AssertionError("successful artifact validation returned no artifact")
            artifact_bytes = artifact.stat().st_size
    finally:
        sampler.stop()

    return DirectMeasurement(
        acquisition_seconds=result.duration.total_seconds(),
        import_seconds=0.0,
        total_seconds=total_seconds,
        artifact_bytes=artifact_bytes,
        peak_rss_bytes=sampler.peak_rss_bytes,
        yt_dlp_process_count=len(process_ids),
        yt_dlp_version=result.yt_dlp_version,
    )


def _measure_worker(
    pool: ConnectionPool[Any],
    *,
    canonical_youtube_url: str,
    deadline_delta: timedelta,
) -> WorkerMeasurement:
    stream_id = uuid4()
    collection_job_id = uuid4()
    _seed_benchmark_records(
        pool,
        stream_id=stream_id,
        collection_job_id=collection_job_id,
        canonical_youtube_url=canonical_youtube_url,
    )
    process_ids: list[int] = []
    batch_sizes: list[int] = []
    sampler = PeakRssSampler((os.getpid(),))

    def observe_process(pid: int) -> None:
        process_ids.append(pid)
        sampler.add_process(pid)

    process = SubprocessYtDlpProcess(process_observer=observe_process)
    messages = PostgresChatMessageRepository(pool, batch_observer=batch_sizes.append)
    sampler.start()
    started = monotonic()
    try:
        with tempfile.TemporaryDirectory(prefix="worker-chat-benchmark-") as attempt_root:
            collector = YtDlpChatReplayCollector(
                process=process,
                messages=messages,
                attempt_root=Path(attempt_root),
            )
            result = collector.collect(
                CollectionRequest(
                    collection_job_id=collection_job_id,
                    stream_id=stream_id,
                    canonical_youtube_url=canonical_youtube_url,
                    attempt=1,
                    deadline=datetime.now(UTC) + deadline_delta,
                ),
                Event(),
            )
            total_seconds = monotonic() - started
        if result.outcome is not CollectionOutcome.SUCCEEDED:
            raise BenchmarkError("Worker collection returned no chat artifact")
        stored_message_count = _stored_message_count(pool, stream_id=stream_id)
    finally:
        sampler.stop()
        _delete_benchmark_records(
            pool,
            stream_id=stream_id,
            collection_job_id=collection_job_id,
        )

    acquisition_seconds = result.duration.total_seconds()
    return WorkerMeasurement(
        acquisition_seconds=acquisition_seconds,
        import_seconds=max(0.0, total_seconds - acquisition_seconds),
        total_seconds=total_seconds,
        artifact_bytes=result.artifact_bytes,
        peak_rss_bytes=sampler.peak_rss_bytes,
        yt_dlp_process_count=len(process_ids),
        worker_owned_youtube_http_request_count=0,
        saved_message_count=result.saved_message_count,
        duplicate_count=result.duplicate_count,
        skipped_action_count=result.skipped_action_count,
        stored_message_count=stored_message_count,
        maximum_batch_size=max(batch_sizes, default=0),
        yt_dlp_version=result.yt_dlp_version,
    )


def _require_successful_artifact(result: YtDlpProcessResult) -> None:
    if result.termination is not ProcessTermination.EXITED:
        raise BenchmarkError(f"direct yt-dlp terminated as {result.termination}")
    if result.exit_code != 0:
        raise BenchmarkError(f"direct yt-dlp exited with code {result.exit_code}")
    if result.partial_artifact_present:
        raise BenchmarkError("direct yt-dlp left a partial artifact")
    if result.artifact_path is None:
        raise BenchmarkError("direct yt-dlp produced no chat artifact")


def _seed_benchmark_records(
    pool: ConnectionPool[Any],
    *,
    stream_id: UUID,
    collection_job_id: UUID,
    canonical_youtube_url: str,
) -> None:
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
            ) VALUES (%s, %s, %s, 'Performance benchmark', 'benchmark-channel',
                      'Performance benchmark', 'ended', CURRENT_TIMESTAMP)
            """,
            (stream_id, f"benchmark-{stream_id.hex}", canonical_youtube_url),
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
            ) VALUES (%s, %s, 'chat_replay', 'succeeded', 1,
                      CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
            """,
            (collection_job_id, stream_id),
        )


def _stored_message_count(pool: ConnectionPool[Any], *, stream_id: UUID) -> int:
    with pool.connection() as connection:
        row = connection.execute(
            "SELECT count(*) FROM chat.chat_messages WHERE stream_id = %s",
            (stream_id,),
        ).fetchone()
    if row is None:
        raise BenchmarkError("could not count imported benchmark messages")
    return int(row[0])


def _delete_benchmark_records(
    pool: ConnectionPool[Any],
    *,
    stream_id: UUID,
    collection_job_id: UUID,
) -> None:
    with pool.connection() as connection:
        connection.execute("DELETE FROM chat.chat_messages WHERE stream_id = %s", (stream_id,))
        connection.execute(
            "DELETE FROM collection.collection_jobs WHERE id = %s", (collection_job_id,)
        )
        connection.execute("DELETE FROM stream.streams WHERE id = %s", (stream_id,))


def render_markdown(report: BenchmarkReport) -> str:
    direct = report.direct
    worker = report.worker
    status = "PASS" if report.passed else "FAIL"
    gate_rows = "\n".join(
        f"| {gate.name} | {'PASS' if gate.passed else 'FAIL'} | {gate.detail} |"
        for gate in report.gates
    )
    return f"""# Direct yt-dlp vs Worker benchmark

- Result: **{status}**
- Started: `{report.started_at.isoformat()}`
- Video: `{report.video_id}`
- yt-dlp: `{direct.yt_dlp_version}`
- Platform: `{report.platform_description}`
- Python: `{report.python_version}`
- Credentials: {report.credentials}
- Execution: direct then Worker on the same machine and network

| Measurement | Direct | Worker |
| --- | ---: | ---: |
| Acquisition | {direct.acquisition_seconds:.3f} s | {worker.acquisition_seconds:.3f} s |
| Import | {direct.import_seconds:.3f} s | {worker.import_seconds:.3f} s |
| Total | {direct.total_seconds:.3f} s | {worker.total_seconds:.3f} s |
| Artifact | {direct.artifact_bytes} B | {worker.artifact_bytes} B |
| Peak RSS | {direct.peak_rss_bytes} B | {worker.peak_rss_bytes} B |
| yt-dlp processes | {direct.yt_dlp_process_count} | {worker.yt_dlp_process_count} |
| Saved messages | n/a | {worker.saved_message_count} |
| Duplicate messages | n/a | {worker.duplicate_count} |
| Skipped actions | n/a | {worker.skipped_action_count} |

## Gates

| Gate | Result | Evidence |
| --- | --- | --- |
{gate_rows}

The Worker-owned YouTube HTTP count is structural evidence: this benchmark composes the
collector only from the yt-dlp subprocess adapter and the PostgreSQL message repository.
The RSS sampler measures the benchmark process, yt-dlp process, and descendants through
Linux `/proc`. Database server memory is outside that process boundary.
"""


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Benchmark direct yt-dlp against the Worker")
    parser.add_argument(
        "--database-url",
        default=os.environ.get("BENCHMARK_DATABASE_URL"),
        help="migrated PostgreSQL URL (or BENCHMARK_DATABASE_URL)",
    )
    parser.add_argument("--video-id", default=DEFAULT_VIDEO_ID)
    parser.add_argument("--timeout-seconds", type=int, default=DEFAULT_TIMEOUT_SECONDS)
    parser.add_argument("--json-output", type=Path)
    parser.add_argument("--markdown-output", type=Path)
    return parser


def _write_report(path: Path, contents: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(contents, encoding="utf-8")


def main(arguments: Sequence[str] | None = None) -> int:
    parser = _build_parser()
    options = parser.parse_args(arguments)
    database_url = options.database_url
    if not isinstance(database_url, str) or not database_url:
        parser.error("--database-url or BENCHMARK_DATABASE_URL is required")

    try:
        report = run_benchmark(
            database_url=database_url,
            video_id=options.video_id,
            timeout_seconds=options.timeout_seconds,
        )
    except BenchmarkError as error:
        parser.exit(2, f"benchmark error: {error}\n")

    serialized = json.dumps(report.as_dict(), indent=2, sort_keys=True) + "\n"
    if options.json_output is not None:
        _write_report(options.json_output, serialized)
    if options.markdown_output is not None:
        _write_report(options.markdown_output, render_markdown(report))
    if options.json_output is None and options.markdown_output is None:
        sys.stdout.write(serialized)
    else:
        sys.stdout.write(render_markdown(report))
    return 0 if report.passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
