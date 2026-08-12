CREATE SCHEMA IF NOT EXISTS collection;

CREATE TABLE collection.collection_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stream_id UUID NOT NULL REFERENCES stream.streams (id),
    kind TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    attempt INTEGER NOT NULL DEFAULT 0,
    worker_id TEXT,
    lease_token UUID,
    lease_expires_at TIMESTAMPTZ,
    heartbeat_at TIMESTAMPTZ,
    processed_count BIGINT NOT NULL DEFAULT 0,
    skipped_count BIGINT NOT NULL DEFAULT 0,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    error_code TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT collection_jobs_kind_check CHECK (kind IN ('chat_replay')),
    CONSTRAINT collection_jobs_status_check
        CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    CONSTRAINT collection_jobs_attempt_nonnegative_check CHECK (attempt >= 0),
    CONSTRAINT collection_jobs_processed_count_nonnegative_check CHECK (processed_count >= 0),
    CONSTRAINT collection_jobs_skipped_count_nonnegative_check CHECK (skipped_count >= 0),
    CONSTRAINT collection_jobs_worker_id_not_empty_check
        CHECK (worker_id IS NULL OR char_length(worker_id) > 0),
    CONSTRAINT collection_jobs_lease_order_check
        CHECK (lease_expires_at IS NULL OR heartbeat_at IS NULL OR lease_expires_at > heartbeat_at),
    CONSTRAINT collection_jobs_running_fields_check CHECK (
        status <> 'running'
        OR (
            worker_id IS NOT NULL
            AND lease_token IS NOT NULL
            AND lease_expires_at IS NOT NULL
            AND heartbeat_at IS NOT NULL
            AND started_at IS NOT NULL
            AND finished_at IS NULL
        )
    ),
    CONSTRAINT collection_jobs_nonrunning_lease_check CHECK (
        status = 'running'
        OR (
            worker_id IS NULL
            AND lease_token IS NULL
            AND lease_expires_at IS NULL
        )
    ),
    CONSTRAINT collection_jobs_terminal_fields_check CHECK (
        status NOT IN ('succeeded', 'failed')
        OR (started_at IS NOT NULL AND finished_at IS NOT NULL)
    ),
    CONSTRAINT collection_jobs_error_fields_check CHECK (
        (status = 'failed' AND error_code IS NOT NULL AND char_length(error_code) > 0)
        OR (status <> 'failed' AND error_code IS NULL AND error_message IS NULL)
    )
);

CREATE UNIQUE INDEX collection_jobs_one_active_per_stream_kind_idx
    ON collection.collection_jobs (stream_id, kind)
    WHERE status IN ('queued', 'running');

CREATE INDEX collection_jobs_claim_idx
    ON collection.collection_jobs (requested_at, id)
    WHERE status = 'queued';

CREATE INDEX collection_jobs_expired_lease_idx
    ON collection.collection_jobs (lease_expires_at, requested_at, id)
    WHERE status = 'running';

CREATE TABLE collection.collection_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES collection.collection_jobs (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    processed_count BIGINT NOT NULL DEFAULT 0,
    skipped_count BIGINT NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    error_code TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT collection_steps_job_name_key UNIQUE (job_id, name),
    CONSTRAINT collection_steps_name_check CHECK (name IN ('metadata', 'chat_replay')),
    CONSTRAINT collection_steps_status_check
        CHECK (status IN ('pending', 'running', 'succeeded', 'failed')),
    CONSTRAINT collection_steps_processed_count_nonnegative_check CHECK (processed_count >= 0),
    CONSTRAINT collection_steps_skipped_count_nonnegative_check CHECK (skipped_count >= 0),
    CONSTRAINT collection_steps_time_check CHECK (
        (status = 'pending' AND started_at IS NULL AND finished_at IS NULL)
        OR (status = 'running' AND started_at IS NOT NULL AND finished_at IS NULL)
        OR (status IN ('succeeded', 'failed') AND started_at IS NOT NULL AND finished_at IS NOT NULL)
    ),
    CONSTRAINT collection_steps_error_fields_check CHECK (
        (status = 'failed' AND error_code IS NOT NULL AND char_length(error_code) > 0)
        OR (status <> 'failed' AND error_code IS NULL AND error_message IS NULL)
    )
);

CREATE INDEX collection_steps_job_id_idx ON collection.collection_steps (job_id, id);
