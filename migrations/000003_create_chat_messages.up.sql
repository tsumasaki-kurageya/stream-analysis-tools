CREATE SCHEMA IF NOT EXISTS chat;

CREATE TABLE chat.chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stream_id UUID NOT NULL REFERENCES stream.streams (id),
    collection_job_id UUID NOT NULL REFERENCES collection.collection_jobs (id),
    source TEXT NOT NULL,
    external_message_id TEXT NOT NULL,
    author_channel_id TEXT,
    author_display_name TEXT NOT NULL,
    message_text TEXT NOT NULL,
    published_at TIMESTAMPTZ NOT NULL,
    offset_milliseconds BIGINT NOT NULL,
    message_type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chat_messages_source_check CHECK (source IN ('youtube_chat_replay')),
    CONSTRAINT chat_messages_external_message_id_not_empty_check
        CHECK (char_length(external_message_id) > 0),
    CONSTRAINT chat_messages_author_display_name_not_empty_check
        CHECK (char_length(author_display_name) > 0),
    CONSTRAINT chat_messages_message_text_not_empty_check CHECK (char_length(message_text) > 0),
    CONSTRAINT chat_messages_message_type_check CHECK (message_type IN ('text')),
    CONSTRAINT chat_messages_stream_source_external_message_key
        UNIQUE (stream_id, source, external_message_id)
);

CREATE INDEX chat_messages_stream_timeline_idx
    ON chat.chat_messages (stream_id, offset_milliseconds, published_at, external_message_id);

CREATE INDEX chat_messages_collection_job_id_idx
    ON chat.chat_messages (collection_job_id);
