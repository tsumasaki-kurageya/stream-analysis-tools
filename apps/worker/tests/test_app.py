import json
import os
from datetime import UTC, datetime, timedelta
from pathlib import Path

import pytest

from stream_analysis_worker.app import build_startup_message, main, run_startup


def test_startup_message_identifies_ready_worker() -> None:
    message = json.loads(build_startup_message())

    assert message == {"component": "collection-worker", "status": "ready"}


def test_startup_reports_cleanup_and_disk_capacity(tmp_path: Path) -> None:
    orphan = tmp_path / "20000000-0000-0000-0000-000000000001-attempt-1-orphan"
    orphan.mkdir()
    (orphan / "chat.live_chat.json").write_bytes(b"temporary-chat")
    now = datetime(2026, 8, 15, 12, tzinfo=UTC)
    old = (now - timedelta(days=2)).timestamp()
    orphan.touch()
    os.utime(orphan, (old, old))

    messages = run_startup(
        attempt_root=tmp_path,
        now=now,
        orphan_after=timedelta(hours=24),
        minimum_free_bytes=1,
    )

    events = [json.loads(message) for message in messages]
    assert [event["event"] for event in events[:2]] == [
        "temporary_artifact_cleanup",
        "disk_capacity",
    ]
    assert events[0]["removed_directory_count"] == 1
    assert events[0]["removed_artifact_bytes"] == len(b"temporary-chat")
    assert events[2] == {"component": "collection-worker", "status": "ready"}


def test_main_reports_capacity_failure_without_a_traceback(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    monkeypatch.setenv("YSA_WORKER_ATTEMPT_ROOT", str(tmp_path))
    monkeypatch.setenv("YSA_WORKER_MINIMUM_FREE_BYTES", str(2**63 - 1))

    with pytest.raises(SystemExit) as caught:
        main()

    assert caught.value.code == 1
    output = capsys.readouterr()
    assert output.err == ""
    events = [json.loads(line) for line in output.out.splitlines()]
    assert events[1]["event"] == "disk_capacity"
    assert events[1]["capacity_ok"] is False
    assert events[2] == {
        "component": "collection-worker",
        "error_code": "WORKER_STARTUP_FAILED",
        "status": "failed",
    }
    assert str(tmp_path) not in output.out
