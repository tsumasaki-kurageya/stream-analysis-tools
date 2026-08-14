from dataclasses import dataclass
from datetime import datetime, timedelta
from enum import StrEnum
from pathlib import Path
from threading import Event
from typing import Protocol


class ProcessTermination(StrEnum):
    EXITED = "exited"
    TIMED_OUT = "timed_out"
    CANCELLED = "cancelled"


@dataclass(frozen=True, slots=True)
class YtDlpProcessRequest:
    canonical_youtube_url: str
    attempt_directory: Path
    deadline: datetime

    def __post_init__(self) -> None:
        if not self.canonical_youtube_url:
            raise ValueError("canonical_youtube_url must not be empty")
        if self.deadline.tzinfo is None or self.deadline.utcoffset() is None:
            raise ValueError("deadline must be timezone-aware")


@dataclass(frozen=True, slots=True)
class YtDlpProcessResult:
    exit_code: int
    termination: ProcessTermination
    artifact_path: Path | None
    stderr: str
    yt_dlp_version: str
    duration: timedelta

    def __post_init__(self) -> None:
        if not self.yt_dlp_version:
            raise ValueError("yt_dlp_version must not be empty")
        if self.duration < timedelta(0):
            raise ValueError("duration must not be negative")


class YtDlpProcessAdapter(Protocol):
    """Internal seam implemented by production and scripted process Adapters."""

    def run(
        self,
        request: YtDlpProcessRequest,
        cancellation: Event,
    ) -> YtDlpProcessResult: ...
