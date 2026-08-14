import os
import signal
import subprocess
import sys
from collections.abc import Sequence
from contextlib import suppress
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from enum import StrEnum
from importlib.metadata import version
from pathlib import Path
from threading import Event
from time import monotonic
from typing import Protocol

_SIGKILL = 9


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
    partial_artifact_present: bool = False

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


class SubprocessYtDlpProcess:
    """Pinned yt-dlp Adapter with controlled argv, paths, and process lifetime."""

    def __init__(
        self,
        *,
        command_prefix: Sequence[str] | None = None,
        poll_interval: float = 0.1,
        termination_grace: float = 2.0,
        stderr_limit_bytes: int = 65_536,
    ) -> None:
        prefix = tuple(command_prefix or (sys.executable, "-m", "yt_dlp"))
        if not prefix or any(not argument for argument in prefix):
            raise ValueError("command_prefix must contain non-empty arguments")
        if poll_interval <= 0:
            raise ValueError("poll_interval must be positive")
        if termination_grace < 0:
            raise ValueError("termination_grace must not be negative")
        if stderr_limit_bytes < 1:
            raise ValueError("stderr_limit_bytes must be positive")
        self._command_prefix = prefix
        self._poll_interval = poll_interval
        self._termination_grace = termination_grace
        self._stderr_limit_bytes = stderr_limit_bytes

    def run(
        self,
        request: YtDlpProcessRequest,
        cancellation: Event,
    ) -> YtDlpProcessResult:
        request.attempt_directory.mkdir(parents=True, exist_ok=True)
        temp_directory = request.attempt_directory / "temp"
        temp_directory.mkdir(exist_ok=True)
        stderr_path = request.attempt_directory / "yt-dlp.stderr"
        arguments = self._arguments(request, temp_directory=temp_directory)
        started_at = monotonic()

        with stderr_path.open("wb") as stderr:
            process = subprocess.Popen(
                arguments,
                stdin=subprocess.DEVNULL,
                stdout=subprocess.DEVNULL,
                stderr=stderr,
                shell=False,
                start_new_session=os.name != "nt",
                creationflags=(subprocess.CREATE_NEW_PROCESS_GROUP if os.name == "nt" else 0),
            )
            termination = self._wait_for_process(
                process,
                deadline=request.deadline,
                cancellation=cancellation,
            )

        artifacts = sorted(request.attempt_directory.glob("*.live_chat.json"))
        artifact_path = artifacts[0] if len(artifacts) == 1 else None
        return YtDlpProcessResult(
            exit_code=process.returncode,
            termination=termination,
            artifact_path=artifact_path,
            stderr=_read_tail(stderr_path, limit=self._stderr_limit_bytes),
            yt_dlp_version=version("yt-dlp"),
            duration=timedelta(seconds=monotonic() - started_at),
            partial_artifact_present=any(temp_directory.rglob("*.part")),
        )

    def _arguments(
        self,
        request: YtDlpProcessRequest,
        *,
        temp_directory: Path,
    ) -> list[str]:
        return [
            *self._command_prefix,
            "--ignore-config",
            "--no-plugin-dirs",
            "--no-playlist",
            "--skip-download",
            "--write-subs",
            "--sub-langs",
            "live_chat",
            "--paths",
            f"home:{request.attempt_directory}",
            "--paths",
            f"temp:{temp_directory}",
            "--output",
            "subtitle:%(id)s.%(ext)s",
            "--socket-timeout",
            "30",
            "--fragment-retries",
            "2",
            "--extractor-retries",
            "2",
            "--no-remote-components",
            request.canonical_youtube_url,
        ]

    def _wait_for_process(
        self,
        process: subprocess.Popen[bytes],
        *,
        deadline: datetime,
        cancellation: Event,
    ) -> ProcessTermination:
        while process.poll() is None:
            if cancellation.is_set():
                self._terminate_process_tree(process)
                return ProcessTermination.CANCELLED
            remaining = (deadline - datetime.now(UTC)).total_seconds()
            if remaining <= 0:
                self._terminate_process_tree(process)
                return ProcessTermination.TIMED_OUT
            try:
                process.wait(timeout=min(self._poll_interval, remaining))
            except subprocess.TimeoutExpired:
                continue
        return ProcessTermination.EXITED

    def _terminate_process_tree(self, process: subprocess.Popen[bytes]) -> None:
        if process.poll() is not None:
            return
        if os.name == "nt":
            subprocess.run(
                ["taskkill", "/PID", str(process.pid), "/T", "/F"],
                stdin=subprocess.DEVNULL,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                check=False,
            )
            process.wait(timeout=max(self._termination_grace, 0.1))
            return

        try:
            _signal_process_group(process.pid, int(signal.SIGTERM))
        except ProcessLookupError:
            process.wait()
            return
        try:
            process.wait(timeout=self._termination_grace)
        except subprocess.TimeoutExpired:
            with suppress(ProcessLookupError):
                _signal_process_group(process.pid, _SIGKILL)
            process.wait()


def _read_tail(path: Path, *, limit: int) -> str:
    with path.open("rb") as contents:
        size = contents.seek(0, os.SEEK_END)
        contents.seek(max(0, size - limit))
        return contents.read().decode("utf-8", errors="replace")


def _signal_process_group(pid: int, signal_number: int) -> None:
    kill_process_group = getattr(os, "killpg", None)
    if kill_process_group is None:
        raise RuntimeError("process-group signaling is unavailable")
    kill_process_group(pid, signal_number)
