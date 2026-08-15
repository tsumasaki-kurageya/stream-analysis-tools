from datetime import UTC, datetime
from pathlib import Path

from stream_analysis_worker.benchmark import (
    MAX_WORKER_RSS_BYTES,
    BenchmarkReport,
    DirectMeasurement,
    PeakRssSampler,
    WorkerMeasurement,
    evaluate_gates,
    render_markdown,
)


def direct_measurement() -> DirectMeasurement:
    return DirectMeasurement(
        acquisition_seconds=100.0,
        import_seconds=0.0,
        total_seconds=100.0,
        artifact_bytes=1_000,
        peak_rss_bytes=100_000,
        yt_dlp_process_count=1,
        yt_dlp_version="2026.7.4",
    )


def worker_measurement(**overrides: object) -> WorkerMeasurement:
    values: dict[str, object] = {
        "acquisition_seconds": 101.0,
        "import_seconds": 4.0,
        "total_seconds": 105.0,
        "artifact_bytes": 1_000,
        "peak_rss_bytes": 200_000,
        "yt_dlp_process_count": 1,
        "worker_owned_youtube_http_request_count": 0,
        "saved_message_count": 10,
        "duplicate_count": 0,
        "skipped_action_count": 2,
        "stored_message_count": 10,
        "maximum_batch_size": 10,
        "yt_dlp_version": "2026.7.4",
    }
    values.update(overrides)
    return WorkerMeasurement(**values)  # type: ignore[arg-type]


def test_performance_gates_pass_at_the_documented_thresholds() -> None:
    gates = evaluate_gates(direct_measurement(), worker_measurement(total_seconds=185.0))

    assert all(gate.passed for gate in gates)
    assert next(gate for gate in gates if gate.name == "worker_total_duration").detail == (
        "actual=185.000s; limit=185.000s"
    )


def test_performance_gates_report_each_failed_invariant() -> None:
    gates = evaluate_gates(
        direct_measurement(),
        worker_measurement(
            total_seconds=185.001,
            peak_rss_bytes=MAX_WORKER_RSS_BYTES,
            yt_dlp_process_count=2,
            worker_owned_youtube_http_request_count=1,
            stored_message_count=9,
            maximum_batch_size=501,
            yt_dlp_version="different",
        ),
    )

    assert {gate.name for gate in gates if not gate.passed} == {
        "same_yt_dlp_version",
        "worker_total_duration",
        "worker_peak_rss",
        "one_yt_dlp_process_per_attempt",
        "zero_worker_owned_youtube_http_requests",
        "stored_message_count",
        "maximum_import_batch_size",
    }


def test_peak_rss_sampler_sums_each_process_in_the_tree_once(tmp_path: Path) -> None:
    write_proc_process(tmp_path, pid=1, rss_kibibytes=10, children=(2, 3))
    write_proc_process(tmp_path, pid=2, rss_kibibytes=20, children=(3,))
    write_proc_process(tmp_path, pid=3, rss_kibibytes=30)
    sampler = PeakRssSampler((1,), proc_root=tmp_path, interval_seconds=1.0)

    sampler.start()
    sampler.stop()

    assert sampler.peak_rss_bytes == 60 * 1024


def test_report_serialization_and_markdown_include_reproducibility_evidence() -> None:
    direct = direct_measurement()
    worker = worker_measurement()
    report = BenchmarkReport(
        started_at=datetime(2026, 8, 15, 12, tzinfo=UTC),
        video_id="fixture-video",
        canonical_youtube_url="https://www.youtube.com/watch?v=fixture-video",
        platform_description="fixture-platform",
        python_version="3.14.0",
        credentials="none",
        direct=direct,
        worker=worker,
        gates=evaluate_gates(direct, worker),
    )

    serialized = report.as_dict()
    markdown = render_markdown(report)

    assert serialized["passed"] is True
    assert serialized["environment"] == {
        "platform": "fixture-platform",
        "python_version": "3.14.0",
        "credentials": "none",
        "same_machine": True,
        "same_network": True,
        "execution_order": ["direct", "worker"],
    }
    assert "Result: **PASS**" in markdown
    assert "worker_total_duration" in markdown
    assert "Worker-owned YouTube HTTP count is structural evidence" in markdown


def write_proc_process(
    proc_root: Path,
    *,
    pid: int,
    rss_kibibytes: int,
    children: tuple[int, ...] = (),
) -> None:
    process_root = proc_root / str(pid)
    task_root = process_root / "task" / str(pid)
    task_root.mkdir(parents=True)
    (process_root / "status").write_text(
        f"Name:\tfixture\nVmRSS:\t{rss_kibibytes} kB\n",
        encoding="ascii",
    )
    (task_root / "children").write_text(
        " ".join(str(child) for child in children),
        encoding="ascii",
    )
