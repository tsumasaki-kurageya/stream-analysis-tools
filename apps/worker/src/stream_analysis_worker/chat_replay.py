import json
import shutil
import tempfile
from collections.abc import Iterator, Sequence
from datetime import UTC, datetime
from pathlib import Path
from threading import Event
from time import monotonic
from typing import Any, Protocol

from stream_analysis_worker.chat import ChatMessage
from stream_analysis_worker.collector import (
    CollectionCancelled,
    CollectionErrorCode,
    CollectionFailure,
    CollectionOutcome,
    CollectionRequest,
    CollectionResult,
)
from stream_analysis_worker.observability import (
    ArtifactDirectoryManager,
    CollectionAttemptMetric,
    InsufficientDiskCapacity,
    MetricSink,
    NullMetricSink,
)
from stream_analysis_worker.yt_dlp_process import (
    ProcessTermination,
    YtDlpFailureReason,
    YtDlpProcessAdapter,
    YtDlpProcessRequest,
)


class ChatMessageRepository(Protocol):
    def upsert_batch(self, messages: Sequence[ChatMessage]) -> int: ...


class ArtifactFormatError(RuntimeError):
    pass


class YtDlpChatReplayCollector:
    def __init__(
        self,
        *,
        process: YtDlpProcessAdapter,
        messages: ChatMessageRepository,
        attempt_root: Path,
        batch_size: int = 500,
        metric_sink: MetricSink | None = None,
        artifact_manager: ArtifactDirectoryManager | None = None,
    ) -> None:
        if batch_size < 1 or batch_size > 500:
            raise ValueError("batch_size must be between 1 and 500")
        self._process = process
        self._messages = messages
        self._attempt_root = attempt_root
        self._batch_size = batch_size
        self._metric_sink = metric_sink or NullMetricSink()
        self._artifact_manager = artifact_manager or ArtifactDirectoryManager(
            root=attempt_root,
            sink=self._metric_sink,
        )

    def collect(
        self,
        request: CollectionRequest,
        cancellation: Event,
    ) -> CollectionResult:
        started_at = monotonic()
        process_result = None
        attempt_directory: Path | None = None
        try:
            try:
                self._artifact_manager.prepare()
            except InsufficientDiskCapacity:
                raise CollectionFailure(
                    code=CollectionErrorCode.WORKER_DISK_CAPACITY_LOW,
                    retryable=True,
                    safe_message="Worker temporary storage capacity is too low.",
                ) from None
            attempt_directory = Path(
                tempfile.mkdtemp(
                    prefix=f"{request.collection_job_id}-attempt-{request.attempt}-",
                    dir=self._artifact_manager.root,
                )
            )
            try:
                process_result = self._process.run(
                    YtDlpProcessRequest(
                        canonical_youtube_url=request.canonical_youtube_url,
                        attempt_directory=attempt_directory,
                        deadline=request.deadline,
                    ),
                    cancellation,
                )
            except CollectionCancelled:
                raise
            except Exception:
                raise CollectionFailure(
                    code=CollectionErrorCode.YTDLP_PROCESS_FAILED,
                    retryable=True,
                    safe_message="yt-dlp failed while collecting chat replay.",
                ) from None
            if process_result.termination is ProcessTermination.TIMED_OUT:
                raise CollectionFailure(
                    code=CollectionErrorCode.YTDLP_TIMEOUT,
                    retryable=True,
                    safe_message="yt-dlp exceeded the chat replay collection deadline.",
                )
            if process_result.termination is ProcessTermination.CANCELLED:
                raise CollectionCancelled("chat replay collection was cancelled")
            if process_result.failure_reason is YtDlpFailureReason.ACCESS_DENIED:
                raise CollectionFailure(
                    code=CollectionErrorCode.YOUTUBE_ACCESS_DENIED,
                    retryable=False,
                    safe_message="YouTube denied access to this video.",
                )
            if process_result.failure_reason is YtDlpFailureReason.REPLAY_NOT_AVAILABLE:
                raise CollectionFailure(
                    code=CollectionErrorCode.CHAT_REPLAY_NOT_AVAILABLE,
                    retryable=False,
                    safe_message="Chat replay is not available for this stream.",
                )
            if process_result.failure_reason is YtDlpFailureReason.SOURCE_NOT_READY:
                raise CollectionFailure(
                    code=CollectionErrorCode.SOURCE_NOT_READY,
                    retryable=True,
                    safe_message="The stream archive is not ready yet.",
                )
            if process_result.exit_code != 0:
                raise CollectionFailure(
                    code=CollectionErrorCode.YTDLP_PROCESS_FAILED,
                    retryable=True,
                    safe_message="yt-dlp failed while collecting chat replay.",
                )
            if process_result.partial_artifact_present:
                raise CollectionFailure(
                    code=CollectionErrorCode.YTDLP_OUTPUT_CHANGED,
                    retryable=False,
                    safe_message="yt-dlp left an incomplete chat artifact.",
                )
            if process_result.artifact_path is None:
                result = CollectionResult(
                    outcome=CollectionOutcome.NO_DATA,
                    saved_message_count=0,
                    duplicate_count=0,
                    skipped_action_count=0,
                    artifact_bytes=0,
                    yt_dlp_version=process_result.yt_dlp_version,
                    duration=process_result.duration,
                )
                self._emit_attempt(request, result=result)
                return result

            saved_count = 0
            duplicate_count = 0
            skipped_count = 0
            batch: list[ChatMessage] = []
            try:
                for message in parse_chat_artifact(
                    process_result.artifact_path,
                    request=request,
                    cancellation=cancellation,
                ):
                    if message is None:
                        skipped_count += 1
                        continue
                    batch.append(message)
                    if len(batch) == self._batch_size:
                        saved = self._messages.upsert_batch(batch)
                        saved_count += saved
                        duplicate_count += len(batch) - saved
                        batch.clear()
                if batch:
                    saved = self._messages.upsert_batch(batch)
                    saved_count += saved
                    duplicate_count += len(batch) - saved
            except CollectionCancelled:
                raise
            except ArtifactFormatError:
                raise CollectionFailure(
                    code=CollectionErrorCode.YTDLP_OUTPUT_CHANGED,
                    retryable=False,
                    safe_message="yt-dlp produced an unsupported chat artifact.",
                ) from None
            except Exception:
                raise CollectionFailure(
                    code=CollectionErrorCode.CHAT_IMPORT_FAILED,
                    retryable=True,
                    safe_message="Chat messages could not be imported.",
                ) from None

            result = CollectionResult(
                outcome=CollectionOutcome.SUCCEEDED,
                saved_message_count=saved_count,
                duplicate_count=duplicate_count,
                skipped_action_count=skipped_count,
                artifact_bytes=process_result.artifact_path.stat().st_size,
                yt_dlp_version=process_result.yt_dlp_version,
                duration=process_result.duration,
            )
            self._emit_attempt(request, result=result)
            return result
        except CollectionFailure as failure:
            self._metric_sink.emit(
                CollectionAttemptMetric(
                    job_id=str(request.collection_job_id),
                    attempt=request.attempt,
                    outcome="failed",
                    duration_seconds=(
                        process_result.duration.total_seconds()
                        if process_result is not None
                        else monotonic() - started_at
                    ),
                    saved_message_count=0,
                    duplicate_count=0,
                    skipped_action_count=0,
                    artifact_bytes=0,
                    yt_dlp_version=(
                        process_result.yt_dlp_version if process_result is not None else None
                    ),
                    error_code=failure.code.value,
                )
            )
            raise
        except CollectionCancelled:
            self._metric_sink.emit(
                CollectionAttemptMetric(
                    job_id=str(request.collection_job_id),
                    attempt=request.attempt,
                    outcome="cancelled",
                    duration_seconds=(
                        process_result.duration.total_seconds()
                        if process_result is not None
                        else monotonic() - started_at
                    ),
                    saved_message_count=0,
                    duplicate_count=0,
                    skipped_action_count=0,
                    artifact_bytes=0,
                    yt_dlp_version=(
                        process_result.yt_dlp_version if process_result is not None else None
                    ),
                    error_code="COLLECTION_CANCELLED",
                )
            )
            raise
        finally:
            if attempt_directory is not None:
                shutil.rmtree(attempt_directory, ignore_errors=True)

    def _emit_attempt(
        self,
        request: CollectionRequest,
        *,
        result: CollectionResult,
    ) -> None:
        self._metric_sink.emit(
            CollectionAttemptMetric(
                job_id=str(request.collection_job_id),
                attempt=request.attempt,
                outcome=result.outcome.value,
                duration_seconds=result.duration.total_seconds(),
                saved_message_count=result.saved_message_count,
                duplicate_count=result.duplicate_count,
                skipped_action_count=result.skipped_action_count,
                artifact_bytes=result.artifact_bytes,
                yt_dlp_version=result.yt_dlp_version,
                error_code=None,
            )
        )


def parse_chat_artifact(
    artifact_path: Path,
    *,
    request: CollectionRequest,
    cancellation: Event,
) -> Iterator[ChatMessage | None]:
    with artifact_path.open(encoding="utf-8") as artifact:
        for line in artifact:
            if cancellation.is_set():
                raise CollectionCancelled("chat replay collection was cancelled")
            if not line.strip():
                continue
            try:
                record: Any = json.loads(line)
                replay = _required_mapping(record, "replayChatItemAction")
                offset_milliseconds = int(_required_string(replay, "videoOffsetTimeMsec"))
                actions = replay.get("actions")
                if not isinstance(actions, list):
                    raise ValueError("actions must be a list")
                for action in actions:
                    yield _parse_action(
                        action,
                        request=request,
                        offset_milliseconds=offset_milliseconds,
                    )
            except (json.JSONDecodeError, TypeError, ValueError) as error:
                raise ArtifactFormatError from error


def _parse_action(
    action: Any,
    *,
    request: CollectionRequest,
    offset_milliseconds: int,
) -> ChatMessage | None:
    if not isinstance(action, dict):
        raise ValueError("action must be an object")
    add_item = action.get("addChatItemAction")
    if not isinstance(add_item, dict):
        return None
    item = add_item.get("item")
    if not isinstance(item, dict):
        raise ValueError("chat item must be an object")
    renderer = item.get("liveChatTextMessageRenderer")
    if renderer is None:
        return None
    if not isinstance(renderer, dict):
        raise ValueError("text message renderer must be an object")

    author = _required_mapping(renderer, "authorName")
    message = _required_mapping(renderer, "message")
    timestamp_microseconds = int(_required_string(renderer, "timestampUsec"))
    author_channel_id = renderer.get("authorExternalChannelId")
    if author_channel_id is not None and not isinstance(author_channel_id, str):
        raise ValueError("authorExternalChannelId must be a string")
    return ChatMessage(
        stream_id=request.stream_id,
        collection_job_id=request.collection_job_id,
        external_message_id=_required_string(renderer, "id"),
        author_channel_id=author_channel_id,
        author_display_name=_required_string(author, "simpleText"),
        message_text=_render_runs(message),
        published_at=datetime.fromtimestamp(timestamp_microseconds / 1_000_000, tz=UTC),
        offset_milliseconds=offset_milliseconds,
    )


def _required_mapping(value: Any, key: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise ValueError("value must be an object")
    item = value.get(key)
    if not isinstance(item, dict):
        raise ValueError(f"{key} must be an object")
    return item


def _required_string(value: dict[str, Any], key: str) -> str:
    item = value.get(key)
    if not isinstance(item, str) or not item:
        raise ValueError(f"{key} must be a non-empty string")
    return item


def _render_runs(message: dict[str, Any]) -> str:
    runs = message.get("runs")
    if not isinstance(runs, list):
        raise ValueError("message runs must be a list")
    parts: list[str] = []
    for run in runs:
        if not isinstance(run, dict):
            raise ValueError("message run must be an object")
        if isinstance(run.get("text"), str):
            parts.append(run["text"])
            continue
        emoji = run.get("emoji")
        if not isinstance(emoji, dict):
            raise ValueError("message run must contain text or emoji")
        shortcuts = emoji.get("shortcuts")
        shortcut = (
            next((item for item in shortcuts if isinstance(item, str) and item), None)
            if isinstance(shortcuts, list)
            else None
        )
        emoji_id = emoji.get("emojiId")
        if shortcut is None and isinstance(emoji_id, str) and emoji_id:
            shortcut = emoji_id
        if shortcut is None:
            raise ValueError("emoji run must have a shortcut or ID")
        parts.append(shortcut)
    text = "".join(parts)
    if not text:
        raise ValueError("message text must not be empty")
    return text
