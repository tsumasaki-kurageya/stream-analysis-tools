import json
import os
import signal
import socket
import sys
from collections.abc import Mapping
from dataclasses import asdict, dataclass
from datetime import UTC, datetime, timedelta
from io import StringIO
from pathlib import Path
from threading import Event

from psycopg_pool import ConnectionPool

from stream_analysis_worker.chat import PostgresChatMessageRepository
from stream_analysis_worker.chat_replay import YtDlpChatReplayCollector
from stream_analysis_worker.jobs import PostgresJobRepository
from stream_analysis_worker.observability import (
    ArtifactDirectoryManager,
    InsufficientDiskCapacity,
    JsonMetricSink,
)
from stream_analysis_worker.worker import ChatReplayJobRunner, ClaimLoop
from stream_analysis_worker.yt_dlp_process import SubprocessYtDlpProcess


@dataclass(frozen=True, slots=True)
class WorkerStatus:
    queue_consumption: str
    component: str = "collection-worker"
    status: str = "ready"


@dataclass(frozen=True, slots=True)
class WorkerSettings:
    queue_enabled: bool
    database_url: str | None
    worker_id: str
    attempt_root: Path
    orphan_after: timedelta
    minimum_free_bytes: int
    lease_duration: timedelta
    heartbeat_interval: timedelta
    poll_interval: timedelta
    attempt_timeout: timedelta

    @classmethod
    def from_environment(cls, environment: Mapping[str, str]) -> "WorkerSettings":
        configured = environment.get("YSA_WORKER_QUEUE_ENABLED", "false").lower()
        if configured not in {"true", "false"}:
            raise ValueError("YSA_WORKER_QUEUE_ENABLED must be 'true' or 'false'")
        queue_enabled = configured == "true"
        database_url = environment.get("YSA_DATABASE_URL") or None
        if queue_enabled and database_url is None:
            raise ValueError("YSA_DATABASE_URL is required when queue consumption is enabled")
        lease_duration = _positive_duration(environment, "YSA_WORKER_LEASE_SECONDS", 120)
        heartbeat_interval = _positive_duration(
            environment, "YSA_WORKER_HEARTBEAT_INTERVAL_SECONDS", 30
        )
        if heartbeat_interval >= lease_duration:
            raise ValueError("YSA_WORKER_HEARTBEAT_INTERVAL_SECONDS must be shorter than the lease")
        return cls(
            queue_enabled=queue_enabled,
            database_url=database_url,
            worker_id=environment.get("YSA_WORKER_ID")
            or socket.gethostname()
            or "collection-worker",
            attempt_root=Path(
                environment.get("YSA_WORKER_ATTEMPT_ROOT", "/tmp/stream-analysis-worker")
            ),
            orphan_after=_positive_duration(environment, "YSA_WORKER_ORPHAN_AFTER_SECONDS", 86_400),
            minimum_free_bytes=_positive_integer(
                environment, "YSA_WORKER_MINIMUM_FREE_BYTES", 1024 * 1024 * 1024
            ),
            lease_duration=lease_duration,
            heartbeat_interval=heartbeat_interval,
            poll_interval=_positive_duration(environment, "YSA_WORKER_POLL_INTERVAL_SECONDS", 1),
            attempt_timeout=_positive_duration(
                environment, "YSA_WORKER_ATTEMPT_TIMEOUT_SECONDS", 1_800
            ),
        )


def build_startup_message(*, queue_enabled: bool = False) -> str:
    status = WorkerStatus(queue_consumption="enabled" if queue_enabled else "disabled")
    return json.dumps(asdict(status), separators=(",", ":"), sort_keys=True)


def run_startup(
    *,
    attempt_root: Path,
    now: datetime | None = None,
    orphan_after: timedelta = timedelta(hours=24),
    minimum_free_bytes: int = 1024 * 1024 * 1024,
    queue_enabled: bool = False,
) -> list[str]:
    output = StringIO()
    manager = ArtifactDirectoryManager(
        root=attempt_root,
        sink=JsonMetricSink(output),
        orphan_after=orphan_after,
        minimum_free_bytes=minimum_free_bytes,
    )
    manager.prepare(now=now)
    return [
        *output.getvalue().splitlines(),
        build_startup_message(queue_enabled=queue_enabled),
    ]


def main() -> None:
    try:
        settings = WorkerSettings.from_environment(os.environ)
        manager = ArtifactDirectoryManager(
            root=settings.attempt_root,
            sink=JsonMetricSink(sys.stdout),
            orphan_after=settings.orphan_after,
            minimum_free_bytes=settings.minimum_free_bytes,
        )
        manager.prepare()
    except (InsufficientDiskCapacity, OSError, ValueError):
        _fail_startup()

    cancellation = Event()
    _install_signal_handlers(cancellation)
    if not settings.queue_enabled:
        _write_startup(settings)
        cancellation.wait()
        return

    try:
        assert settings.database_url is not None
        with ConnectionPool(settings.database_url, min_size=1, max_size=4) as pool:
            sink = JsonMetricSink(sys.stdout)
            collector = YtDlpChatReplayCollector(
                process=SubprocessYtDlpProcess(),
                messages=PostgresChatMessageRepository(pool),
                attempt_root=settings.attempt_root,
                metric_sink=sink,
                artifact_manager=manager,
            )
            loop = ClaimLoop(
                repository=PostgresJobRepository(pool),
                runner=ChatReplayJobRunner(
                    collector=collector,
                    clock=lambda: datetime.now(UTC),
                    attempt_timeout=settings.attempt_timeout,
                ),
                worker_id=settings.worker_id,
                lease_duration=settings.lease_duration,
                heartbeat_interval=settings.heartbeat_interval,
                poll_interval=settings.poll_interval,
                clock=lambda: datetime.now(UTC),
                metric_sink=sink,
            )
            _write_startup(settings)
            loop.run_until_cancelled(cancellation)
    except Exception:
        if cancellation.is_set():
            return
        _fail_startup()


def _write_startup(settings: WorkerSettings) -> None:
    sys.stdout.write(build_startup_message(queue_enabled=settings.queue_enabled) + "\n")
    sys.stdout.flush()


def _fail_startup() -> None:
    failure = {
        "component": "collection-worker",
        "error_code": "WORKER_STARTUP_FAILED",
        "status": "failed",
    }
    sys.stdout.write(json.dumps(failure, separators=(",", ":"), sort_keys=True) + "\n")
    sys.stdout.flush()
    raise SystemExit(1) from None


def _install_signal_handlers(cancellation: Event) -> None:
    def cancel(_signal_number: int, _frame: object) -> None:
        cancellation.set()

    signal.signal(signal.SIGTERM, cancel)
    signal.signal(signal.SIGINT, cancel)


def _positive_duration(
    environment: Mapping[str, str], name: str, default_seconds: int
) -> timedelta:
    return timedelta(seconds=_positive_integer(environment, name, default_seconds))


def _positive_integer(environment: Mapping[str, str], name: str, default: int) -> int:
    value = int(environment.get(name, str(default)))
    if value <= 0:
        raise ValueError(f"{name} must be positive")
    return value
