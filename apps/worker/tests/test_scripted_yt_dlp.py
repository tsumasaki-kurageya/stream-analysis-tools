from concurrent.futures import ThreadPoolExecutor
from datetime import UTC, datetime
from pathlib import Path
from threading import Event

from support.scripted_yt_dlp import ScriptedRun, ScriptedYtDlpProcess

from stream_analysis_worker.yt_dlp_process import (
    ProcessTermination,
    YtDlpProcessRequest,
)


def process_request(attempt_directory: Path) -> YtDlpProcessRequest:
    return YtDlpProcessRequest(
        canonical_youtube_url="https://www.youtube.com/watch?v=fixture-video",
        attempt_directory=attempt_directory,
        deadline=datetime(2026, 8, 14, 12, 5, tzinfo=UTC),
    )


def test_scripted_process_copies_artifact_and_records_the_attempt(tmp_path: Path) -> None:
    source = tmp_path / "source.ndjson"
    source.write_text('{"fixture":true}\n', encoding="utf-8")
    process = ScriptedYtDlpProcess([ScriptedRun(artifact_source=source)])

    result = process.run(process_request(tmp_path / "attempt"), Event())

    assert process.run_count == 1
    assert result.termination is ProcessTermination.EXITED
    assert result.artifact_path is not None
    assert result.artifact_path.read_bytes() == source.read_bytes()


def test_scripted_process_waits_for_cancellation_and_records_termination(tmp_path: Path) -> None:
    process = ScriptedYtDlpProcess([ScriptedRun(wait_for_cancellation=True)])
    cancellation = Event()

    with ThreadPoolExecutor(max_workers=1) as executor:
        running = executor.submit(
            process.run,
            process_request(tmp_path / "attempt"),
            cancellation,
        )
        assert process.started.wait(timeout=2)
        cancellation.set()
        result = running.result(timeout=2)

    assert result.termination is ProcessTermination.CANCELLED
    assert process.terminated_process_tree is True
