import json
import os
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor
from datetime import UTC, datetime, timedelta
from pathlib import Path
from threading import Event
from time import sleep

from stream_analysis_worker.yt_dlp_process import (
    ProcessTermination,
    SubprocessYtDlpProcess,
    YtDlpProcessRequest,
)


def test_subprocess_adapter_uses_controlled_argv_and_returns_the_final_artifact(
    tmp_path: Path,
) -> None:
    capture_path = tmp_path / "argv.json"
    fake_yt_dlp = write_success_script(tmp_path / "fake_yt_dlp.py")
    process = SubprocessYtDlpProcess(
        command_prefix=(sys.executable, str(fake_yt_dlp), str(capture_path)),
        poll_interval=0.01,
    )
    attempt_directory = tmp_path / "attempt"
    url = "https://www.youtube.com/watch?v=fixture&list=value;echo-not-a-shell"

    result = process.run(
        YtDlpProcessRequest(
            canonical_youtube_url=url,
            attempt_directory=attempt_directory,
            deadline=datetime.now(UTC) + timedelta(seconds=5),
        ),
        Event(),
    )

    assert json.loads(capture_path.read_text(encoding="utf-8")) == [
        "--ignore-config",
        "--no-plugin-dirs",
        "--no-playlist",
        "--skip-download",
        "--write-subs",
        "--sub-langs",
        "live_chat",
        "--paths",
        f"home:{attempt_directory}",
        "--paths",
        f"temp:{attempt_directory / 'temp'}",
        "--output",
        "subtitle:%(id)s.%(ext)s",
        "--socket-timeout",
        "30",
        "--fragment-retries",
        "2",
        "--extractor-retries",
        "2",
        "--no-remote-components",
        url,
    ]
    assert result.exit_code == 0
    assert result.termination is ProcessTermination.EXITED
    assert result.artifact_path == attempt_directory / "fixture.live_chat.json"
    assert result.artifact_path.read_text(encoding="utf-8") == '{"fixture":true}\n'
    assert result.yt_dlp_version == "2026.7.4"
    assert result.duration >= timedelta(0)


def test_subprocess_adapter_terminates_the_process_tree_at_the_deadline(tmp_path: Path) -> None:
    child_pid_path = tmp_path / "child.pid"
    sleeper = write_process_tree_script(tmp_path / "process_tree.py")
    process = SubprocessYtDlpProcess(
        command_prefix=(sys.executable, str(sleeper), str(child_pid_path)),
        poll_interval=0.01,
        termination_grace=0.2,
    )

    result = process.run(
        YtDlpProcessRequest(
            canonical_youtube_url="https://www.youtube.com/watch?v=fixture",
            attempt_directory=tmp_path / "attempt",
            deadline=datetime.now(UTC) + timedelta(milliseconds=300),
        ),
        Event(),
    )

    assert result.termination is ProcessTermination.TIMED_OUT
    child_pid = int(child_pid_path.read_text(encoding="utf-8"))
    assert_process_stops(child_pid)


def test_subprocess_adapter_terminates_the_process_tree_on_cancellation(tmp_path: Path) -> None:
    child_pid_path = tmp_path / "child.pid"
    sleeper = write_process_tree_script(tmp_path / "process_tree.py")
    process = SubprocessYtDlpProcess(
        command_prefix=(sys.executable, str(sleeper), str(child_pid_path)),
        poll_interval=0.01,
        termination_grace=0.2,
    )
    cancellation = Event()

    with ThreadPoolExecutor(max_workers=1) as executor:
        running = executor.submit(
            process.run,
            YtDlpProcessRequest(
                canonical_youtube_url="https://www.youtube.com/watch?v=fixture",
                attempt_directory=tmp_path / "attempt",
                deadline=datetime.now(UTC) + timedelta(seconds=5),
            ),
            cancellation,
        )
        wait_for_file(child_pid_path)
        cancellation.set()
        result = running.result(timeout=3)

    assert result.termination is ProcessTermination.CANCELLED
    child_pid = int(child_pid_path.read_text(encoding="utf-8"))
    assert_process_stops(child_pid)


def test_subprocess_adapter_bounds_captured_stderr(tmp_path: Path) -> None:
    failure = write_stderr_script(tmp_path / "stderr_failure.py")
    process = SubprocessYtDlpProcess(
        command_prefix=(sys.executable, str(failure)),
        poll_interval=0.01,
        stderr_limit_bytes=128,
    )

    result = process.run(
        YtDlpProcessRequest(
            canonical_youtube_url="https://www.youtube.com/watch?v=fixture",
            attempt_directory=tmp_path / "attempt",
            deadline=datetime.now(UTC) + timedelta(seconds=5),
        ),
        Event(),
    )

    assert result.exit_code == 1
    assert result.stderr == "x" * 128


def write_success_script(path: Path) -> Path:
    path.write_text(
        """
import json
from pathlib import Path
import sys

capture_path = Path(sys.argv[1])
arguments = sys.argv[2:]
capture_path.write_text(json.dumps(arguments), encoding="utf-8")
home = next(value.removeprefix("home:") for value in arguments if value.startswith("home:"))
artifact = Path(home) / "fixture.live_chat.json"
artifact.parent.mkdir(parents=True, exist_ok=True)
artifact.write_text('{"fixture":true}\\n', encoding="utf-8")
""".lstrip(),
        encoding="utf-8",
    )
    return path


def write_process_tree_script(path: Path) -> Path:
    path.write_text(
        """
from pathlib import Path
import subprocess
import sys
import time

child = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(60)"])
Path(sys.argv[1]).write_text(str(child.pid), encoding="utf-8")
time.sleep(60)
""".lstrip(),
        encoding="utf-8",
    )
    return path


def write_stderr_script(path: Path) -> Path:
    path.write_text(
        """
import sys

sys.stderr.write("x" * 10_000)
raise SystemExit(1)
""".lstrip(),
        encoding="utf-8",
    )
    return path


def wait_for_file(path: Path) -> None:
    for _ in range(200):
        if path.exists():
            return
        sleep(0.01)
    raise AssertionError(f"process did not create {path}")


def assert_process_stops(pid: int) -> None:
    for _ in range(200):
        if not process_exists(pid):
            return
        sleep(0.01)
    raise AssertionError(f"child process {pid} is still running")


def process_exists(pid: int) -> bool:
    if os.name == "nt":
        result = subprocess.run(
            ["tasklist", "/FI", f"PID eq {pid}", "/FO", "CSV", "/NH"],
            check=False,
            capture_output=True,
            text=True,
        )
        return f'"{pid}"' in result.stdout
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    return True
