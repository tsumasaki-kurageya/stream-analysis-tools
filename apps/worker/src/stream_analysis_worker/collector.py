from dataclasses import dataclass
from datetime import datetime, timedelta
from enum import StrEnum
from threading import Event
from typing import Protocol
from uuid import UUID


class CollectionOutcome(StrEnum):
    SUCCEEDED = "succeeded"
    NO_DATA = "no_data"


class CollectionErrorCode(StrEnum):
    CHAT_REPLAY_NOT_AVAILABLE = "CHAT_REPLAY_NOT_AVAILABLE"
    SOURCE_NOT_READY = "SOURCE_NOT_READY"
    YOUTUBE_ACCESS_DENIED = "YOUTUBE_ACCESS_DENIED"
    YOUTUBE_RATE_LIMITED = "YOUTUBE_RATE_LIMITED"
    YTDLP_TIMEOUT = "YTDLP_TIMEOUT"
    YTDLP_PROCESS_FAILED = "YTDLP_PROCESS_FAILED"
    YTDLP_OUTPUT_CHANGED = "YTDLP_OUTPUT_CHANGED"
    CHAT_IMPORT_FAILED = "CHAT_IMPORT_FAILED"


@dataclass(frozen=True, slots=True)
class CollectionRequest:
    collection_job_id: UUID
    stream_id: UUID
    canonical_youtube_url: str
    attempt: int
    deadline: datetime

    def __post_init__(self) -> None:
        if not self.canonical_youtube_url:
            raise ValueError("canonical_youtube_url must not be empty")
        if self.attempt < 1:
            raise ValueError("attempt must be positive")
        if self.deadline.tzinfo is None or self.deadline.utcoffset() is None:
            raise ValueError("deadline must be timezone-aware")


@dataclass(frozen=True, slots=True)
class CollectionResult:
    outcome: CollectionOutcome
    saved_message_count: int
    duplicate_count: int
    skipped_action_count: int
    artifact_bytes: int
    yt_dlp_version: str
    duration: timedelta

    def __post_init__(self) -> None:
        counts = (
            self.saved_message_count,
            self.duplicate_count,
            self.skipped_action_count,
            self.artifact_bytes,
        )
        if any(value < 0 for value in counts):
            raise ValueError("collection counts and artifact_bytes must not be negative")
        if not self.yt_dlp_version:
            raise ValueError("yt_dlp_version must not be empty")
        if self.duration < timedelta(0):
            raise ValueError("duration must not be negative")
        if self.outcome is CollectionOutcome.NO_DATA and any(counts):
            raise ValueError("no_data results must not report messages, actions, or artifacts")


class CollectionFailure(RuntimeError):
    """A failed attempt containing only safe, durable error details."""

    def __init__(
        self,
        *,
        code: CollectionErrorCode,
        retryable: bool,
        safe_message: str,
    ) -> None:
        if not safe_message:
            raise ValueError("safe_message must not be empty")
        super().__init__(safe_message)
        self.code = code
        self.retryable = retryable
        self.safe_message = safe_message


class CollectionCancelled(RuntimeError):
    """Raised after cancellation has stopped the process tree and cleanup completed."""


class ChatReplayCollector(Protocol):
    """Deep module interface for one bounded chat-replay collection attempt.

    Implementations launch exactly one yt-dlp process, make no Worker-owned
    YouTube requests, persist messages in batches of at most 500, and remove
    attempt artifacts before returning or raising. Expected acquisition and
    import failures raise ``CollectionFailure`` with redacted durable details;
    cooperative shutdown raises ``CollectionCancelled`` after process-tree
    termination and cleanup.

    CLI flags, artifact names, renderer shapes, database batching, and process
    management remain implementation details behind this interface.
    """

    def collect(
        self,
        request: CollectionRequest,
        cancellation: Event,
    ) -> CollectionResult: ...
