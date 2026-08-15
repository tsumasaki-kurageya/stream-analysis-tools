from collections.abc import Sequence
from dataclasses import dataclass
from datetime import timedelta
from pathlib import Path
from shutil import copyfile
from threading import Event

from stream_analysis_worker.yt_dlp_process import (
    ProcessTermination,
    YtDlpFailureReason,
    YtDlpProcessRequest,
    YtDlpProcessResult,
)


@dataclass(frozen=True, slots=True)
class ScriptedRun:
    artifact_source: Path | None = None
    exit_code: int = 0
    termination: ProcessTermination = ProcessTermination.EXITED
    stderr: str = ""
    duration: timedelta = timedelta(milliseconds=250)
    wait_for_cancellation: bool = False
    partial_artifact_present: bool = False
    failure_reason: YtDlpFailureReason | None = None


class ScriptedYtDlpProcess:
    """Deterministic Adapter for the collector's true-external process seam."""

    def __init__(self, scripts: Sequence[ScriptedRun]) -> None:
        if not scripts:
            raise ValueError("at least one scripted run is required")
        self._scripts = list(scripts)
        self.requests: list[YtDlpProcessRequest] = []
        self.started = Event()
        self.terminated_process_tree = False

    @property
    def run_count(self) -> int:
        return len(self.requests)

    def run(
        self,
        request: YtDlpProcessRequest,
        cancellation: Event,
    ) -> YtDlpProcessResult:
        try:
            script = self._scripts[len(self.requests)]
        except IndexError as error:
            raise AssertionError("collector started more yt-dlp processes than scripted") from error

        self.requests.append(request)
        request.attempt_directory.mkdir(parents=True, exist_ok=True)
        self.started.set()

        termination = script.termination
        if script.wait_for_cancellation:
            if not cancellation.wait(timeout=2):
                raise AssertionError("collector did not forward cancellation to yt-dlp")
            termination = ProcessTermination.CANCELLED

        artifact_path: Path | None = None
        if script.artifact_source is not None:
            artifact_path = request.attempt_directory / "scripted-artifact.ndjson"
            copyfile(script.artifact_source, artifact_path)
        if script.partial_artifact_present:
            (request.attempt_directory / "incomplete.live_chat.json.part").touch()

        self.terminated_process_tree |= termination is not ProcessTermination.EXITED
        return YtDlpProcessResult(
            exit_code=script.exit_code,
            termination=termination,
            artifact_path=artifact_path,
            stderr=script.stderr,
            yt_dlp_version="2026.7.4",
            duration=script.duration,
            partial_artifact_present=script.partial_artifact_present,
            failure_reason=script.failure_reason,
        )
