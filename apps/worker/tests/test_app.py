import json

from stream_analysis_worker.app import build_startup_message


def test_startup_message_identifies_ready_worker() -> None:
    message = json.loads(build_startup_message())

    assert message == {"component": "collection-worker", "status": "ready"}
