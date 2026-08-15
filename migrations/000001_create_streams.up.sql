CREATE SCHEMA IF NOT EXISTS stream;

CREATE TABLE stream.streams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    youtube_video_id TEXT NOT NULL,
    canonical_url TEXT NOT NULL,
    title TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    channel_title TEXT NOT NULL,
    thumbnail_url TEXT,
    scheduled_start_at TIMESTAMPTZ,
    actual_start_at TIMESTAMPTZ,
    actual_end_at TIMESTAMPTZ,
    duration_ms BIGINT,
    lifecycle_status TEXT NOT NULL,
    metadata_fetched_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT streams_youtube_video_id_key UNIQUE (youtube_video_id),
    CONSTRAINT streams_youtube_video_id_length_check
        CHECK (char_length(youtube_video_id) BETWEEN 1 AND 64),
    CONSTRAINT streams_canonical_url_not_empty_check CHECK (char_length(canonical_url) > 0),
    CONSTRAINT streams_title_not_empty_check CHECK (char_length(title) > 0),
    CONSTRAINT streams_channel_id_not_empty_check CHECK (char_length(channel_id) > 0),
    CONSTRAINT streams_channel_title_not_empty_check CHECK (char_length(channel_title) > 0),
    CONSTRAINT streams_duration_nonnegative_check CHECK (duration_ms IS NULL OR duration_ms >= 0),
    CONSTRAINT streams_actual_time_order_check
        CHECK (actual_end_at IS NULL OR actual_start_at IS NULL OR actual_end_at >= actual_start_at),
    CONSTRAINT streams_lifecycle_status_check
        CHECK (lifecycle_status IN ('unknown', 'scheduled', 'live', 'ended', 'unavailable'))
);

CREATE INDEX streams_created_at_id_idx ON stream.streams (created_at DESC, id DESC);
