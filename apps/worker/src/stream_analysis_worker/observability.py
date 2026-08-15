import json
import re
import shutil
from dataclasses import asdict, dataclass
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import ClassVar, Protocol, TextIO

_ATTEMPT_DIRECTORY = re.compile(
    r"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}-attempt-[1-9][0-9]*-.+$",
    re.IGNORECASE,
)
_SAFE_CODE = re.compile(r"^[A-Z][A-Z0-9_]{0,63}$")
_BEARER = re.compile(r"(?i)(authorization\s*:\s*bearer\s+)[^\s,;]+")
_COOKIE_HEADER = re.compile(r"(?i)(cookie\s*:\s*)[^\r\n]+")
_SENSITIVE_FLAG = re.compile(
    r"(?i)(--(?:cookies(?:-from-browser)?|proxy)\s+)(?:\"[^\"]*\"|'[^']*'|\S+)"
)
_URL_CREDENTIAL = re.compile(r"(?i)(https?://)[^\s/@:]+:[^\s/@]+@")
_SENSITIVE_QUERY = re.compile(
    r"(?i)([?&](?:authorization|continuation|cookie|proxy|token)=)[^&\s]+"
)


@dataclass(frozen=True, slots=True)
class CollectionAttemptMetric:
    job_id: str
    attempt: int
    outcome: str
    duration_seconds: float
    saved_message_count: int
    duplicate_count: int
    skipped_action_count: int
    artifact_bytes: int
    yt_dlp_version: str | None
    error_code: str | None
    event: ClassVar[str] = "yt_dlp_collection_attempt"

    def __post_init__(self) -> None:
        if self.attempt < 1 or self.duration_seconds < 0:
            raise ValueError("attempt must be positive and duration must not be negative")
        if any(
            value < 0
            for value in (
                self.saved_message_count,
                self.duplicate_count,
                self.skipped_action_count,
                self.artifact_bytes,
            )
        ):
            raise ValueError("metric counts and bytes must not be negative")
        if self.error_code is not None and not _SAFE_CODE.fullmatch(self.error_code):
            raise ValueError("error_code must be a stable uppercase code")


@dataclass(frozen=True, slots=True)
class JobMetric:
    job_id: str
    job_kind: str
    attempt: int
    outcome: str
    duration_seconds: float
    processed_count: int
    skipped_count: int
    error_code: str | None
    event: ClassVar[str] = "collection_job"

    def __post_init__(self) -> None:
        if self.attempt < 1 or self.duration_seconds < 0:
            raise ValueError("attempt must be positive and duration must not be negative")
        if self.processed_count < 0 or self.skipped_count < 0:
            raise ValueError("job metric counts must not be negative")
        if self.error_code is not None and not _SAFE_CODE.fullmatch(self.error_code):
            raise ValueError("error_code must be a stable uppercase code")


@dataclass(frozen=True, slots=True)
class CleanupMetric:
    removed_directory_count: int
    removed_artifact_bytes: int
    event: ClassVar[str] = "temporary_artifact_cleanup"


@dataclass(frozen=True, slots=True)
class DiskCapacityMetric:
    free_bytes: int
    total_bytes: int
    used_bytes: int
    capacity_ok: bool
    event: ClassVar[str] = "disk_capacity"


type Metric = CollectionAttemptMetric | JobMetric | CleanupMetric | DiskCapacityMetric


class MetricSink(Protocol):
    def emit(self, metric: Metric) -> None: ...


class NullMetricSink:
    def emit(self, metric: Metric) -> None:
        del metric


class JsonMetricSink:
    """Writes allowlisted, typed operational metrics as one JSON object per line."""

    def __init__(self, output: TextIO) -> None:
        self._output = output

    def emit(self, metric: Metric) -> None:
        payload = {"event": metric.event, **asdict(metric)}
        self._output.write(json.dumps(payload, separators=(",", ":"), sort_keys=True) + "\n")
        self._output.flush()


@dataclass(frozen=True, slots=True)
class ArtifactCleanupSnapshot:
    removed_directory_count: int
    removed_artifact_bytes: int
    free_bytes: int
    total_bytes: int
    used_bytes: int
    capacity_ok: bool


class InsufficientDiskCapacity(RuntimeError):
    pass


class ArtifactDirectoryManager:
    def __init__(
        self,
        *,
        root: Path,
        sink: MetricSink | None = None,
        orphan_after: timedelta = timedelta(hours=24),
        minimum_free_bytes: int = 1024 * 1024 * 1024,
    ) -> None:
        if orphan_after <= timedelta(0):
            raise ValueError("orphan_after must be positive")
        if minimum_free_bytes < 0:
            raise ValueError("minimum_free_bytes must not be negative")
        self.root = root
        self._sink = sink or NullMetricSink()
        self._orphan_after = orphan_after
        self._minimum_free_bytes = minimum_free_bytes

    def prepare(self, *, now: datetime | None = None) -> ArtifactCleanupSnapshot:
        checked_at = now or datetime.now(UTC)
        if checked_at.tzinfo is None or checked_at.utcoffset() is None:
            raise ValueError("now must be timezone-aware")
        self.root.mkdir(parents=True, exist_ok=True)
        removed_count = 0
        removed_bytes = 0
        cutoff = checked_at.timestamp() - self._orphan_after.total_seconds()
        for candidate in self.root.iterdir():
            if (
                not candidate.is_dir()
                or candidate.is_symlink()
                or not _ATTEMPT_DIRECTORY.fullmatch(candidate.name)
                or candidate.stat().st_mtime >= cutoff
            ):
                continue
            removed_bytes += _directory_bytes(candidate)
            shutil.rmtree(candidate)
            removed_count += 1

        usage = shutil.disk_usage(self.root)
        capacity_ok = usage.free >= self._minimum_free_bytes
        self._sink.emit(
            CleanupMetric(
                removed_directory_count=removed_count,
                removed_artifact_bytes=removed_bytes,
            )
        )
        self._sink.emit(
            DiskCapacityMetric(
                free_bytes=usage.free,
                total_bytes=usage.total,
                used_bytes=usage.used,
                capacity_ok=capacity_ok,
            )
        )
        snapshot = ArtifactCleanupSnapshot(
            removed_directory_count=removed_count,
            removed_artifact_bytes=removed_bytes,
            free_bytes=usage.free,
            total_bytes=usage.total,
            used_bytes=usage.used,
            capacity_ok=capacity_ok,
        )
        if not capacity_ok:
            raise InsufficientDiskCapacity("worker temporary storage capacity is below its limit")
        return snapshot


def redact_text(value: str) -> str:
    """Defense-in-depth for diagnostic text; operational metrics avoid raw text entirely."""

    redacted = _BEARER.sub(r"\1[REDACTED]", value)
    redacted = _COOKIE_HEADER.sub(r"\1[REDACTED]", redacted)
    redacted = _SENSITIVE_FLAG.sub(r"\1[REDACTED]", redacted)
    redacted = _URL_CREDENTIAL.sub(r"\1[REDACTED]@", redacted)
    return _SENSITIVE_QUERY.sub(r"\1[REDACTED]", redacted)


def safe_error_code(value: str) -> str:
    return value if _SAFE_CODE.fullmatch(value) else "INTERNAL_ERROR"


def _directory_bytes(directory: Path) -> int:
    return sum(
        path.stat().st_size
        for path in directory.rglob("*")
        if path.is_file() and not path.is_symlink()
    )
