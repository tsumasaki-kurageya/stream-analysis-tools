from collections.abc import Callable, Sequence
from dataclasses import dataclass
from datetime import datetime
from typing import Any
from uuid import UUID

from psycopg.types.json import Jsonb
from psycopg_pool import ConnectionPool


@dataclass(frozen=True, slots=True)
class ChatMessage:
    stream_id: UUID
    collection_job_id: UUID
    external_message_id: str
    author_channel_id: str | None
    author_display_name: str
    message_text: str
    published_at: datetime
    offset_milliseconds: int
    source: str = "youtube_chat_replay"
    message_type: str = "text"


class PostgresChatMessageRepository:
    def __init__(
        self,
        pool: ConnectionPool[Any],
        *,
        batch_observer: Callable[[int], None] | None = None,
    ) -> None:
        self._pool = pool
        self._batch_observer = batch_observer

    def upsert_batch(self, messages: Sequence[ChatMessage]) -> int:
        if not messages:
            return 0
        if self._batch_observer is not None:
            self._batch_observer(len(messages))

        records = [
            {
                "stream_id": str(message.stream_id),
                "collection_job_id": str(message.collection_job_id),
                "source": message.source,
                "external_message_id": message.external_message_id,
                "author_channel_id": message.author_channel_id,
                "author_display_name": message.author_display_name,
                "message_text": message.message_text,
                "published_at": message.published_at.isoformat(),
                "offset_milliseconds": message.offset_milliseconds,
                "message_type": message.message_type,
            }
            for message in messages
        ]

        with self._pool.connection() as connection:
            inserted = connection.execute(
                """
                INSERT INTO chat.chat_messages (
                    stream_id,
                    collection_job_id,
                    source,
                    external_message_id,
                    author_channel_id,
                    author_display_name,
                    message_text,
                    published_at,
                    offset_milliseconds,
                    message_type
                )
                SELECT
                    record.stream_id,
                    record.collection_job_id,
                    record.source,
                    record.external_message_id,
                    record.author_channel_id,
                    record.author_display_name,
                    record.message_text,
                    record.published_at,
                    record.offset_milliseconds,
                    record.message_type
                FROM jsonb_to_recordset(%(messages)s) AS record (
                    stream_id UUID,
                    collection_job_id UUID,
                    source TEXT,
                    external_message_id TEXT,
                    author_channel_id TEXT,
                    author_display_name TEXT,
                    message_text TEXT,
                    published_at TIMESTAMPTZ,
                    offset_milliseconds BIGINT,
                    message_type TEXT
                )
                ON CONFLICT (stream_id, source, external_message_id) DO NOTHING
                RETURNING id
                """,
                {"messages": Jsonb(records)},
            ).fetchall()
        return len(inserted)
