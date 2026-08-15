import json
import os
import sys
from dataclasses import asdict, dataclass
from datetime import datetime, timedelta
from io import StringIO
from pathlib import Path

from stream_analysis_worker.observability import (
    ArtifactDirectoryManager,
    InsufficientDiskCapacity,
    JsonMetricSink,
)


@dataclass(frozen=True, slots=True)
class WorkerStatus:
    component: str = "collection-worker"
    status: str = "ready"


def build_startup_message() -> str:
    return json.dumps(asdict(WorkerStatus()), separators=(",", ":"), sort_keys=True)


def run_startup(
    *,
    attempt_root: Path,
    now: datetime | None = None,
    orphan_after: timedelta = timedelta(hours=24),
    minimum_free_bytes: int = 1024 * 1024 * 1024,
) -> list[str]:
    output = StringIO()
    manager = ArtifactDirectoryManager(
        root=attempt_root,
        sink=JsonMetricSink(output),
        orphan_after=orphan_after,
        minimum_free_bytes=minimum_free_bytes,
    )
    manager.prepare(now=now)
    return [*output.getvalue().splitlines(), build_startup_message()]


def main() -> None:
    try:
        manager = ArtifactDirectoryManager(
            root=Path(os.getenv("YSA_WORKER_ATTEMPT_ROOT", "/tmp/stream-analysis-worker")),
            sink=JsonMetricSink(sys.stdout),
            orphan_after=timedelta(
                seconds=int(os.getenv("YSA_WORKER_ORPHAN_AFTER_SECONDS", "86400"))
            ),
            minimum_free_bytes=int(
                os.getenv("YSA_WORKER_MINIMUM_FREE_BYTES", str(1024 * 1024 * 1024))
            ),
        )
        manager.prepare()
    except (InsufficientDiskCapacity, OSError, ValueError):
        failure = {
            "component": "collection-worker",
            "error_code": "WORKER_STARTUP_FAILED",
            "status": "failed",
        }
        sys.stdout.write(json.dumps(failure, separators=(",", ":"), sort_keys=True) + "\n")
        raise SystemExit(1) from None
    sys.stdout.write(build_startup_message() + "\n")
