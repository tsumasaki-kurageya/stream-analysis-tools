CREATE SCHEMA IF NOT EXISTS reservation;

CREATE TABLE reservation.reservations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    youtube_video_id TEXT NOT NULL,
    source_url TEXT NOT NULL,
    stream_id UUID REFERENCES stream.streams (id),
    collection_job_id UUID,
    state TEXT NOT NULL,
    scheduled_start_at TIMESTAMPTZ,
    actual_start_at TIMESTAMPTZ,
    actual_end_at TIMESTAMPTZ,
    next_check_at TIMESTAMPTZ NOT NULL,
    last_checked_at TIMESTAMPTZ,
    monitor_attempt INTEGER NOT NULL DEFAULT 0,
    last_error_code TEXT,
    last_error_message TEXT,
    last_error_retryable BOOLEAN,
    worker_id TEXT,
    lease_expires_at TIMESTAMPTZ,
    heartbeat_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 0,
    canceled_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT reservations_youtube_video_id_check CHECK (char_length(youtube_video_id) = 11),
    CONSTRAINT reservations_source_url_check CHECK (char_length(source_url) > 0),
    CONSTRAINT reservations_state_check CHECK (
        state IN (
            'scheduled',
            'monitoring',
            'live',
            'waiting_for_archive',
            'collecting',
            'completed',
            'failed',
            'canceled'
        )
    ),
    CONSTRAINT reservations_monitor_attempt_nonnegative_check CHECK (monitor_attempt >= 0),
    CONSTRAINT reservations_revision_nonnegative_check CHECK (revision >= 0),
    CONSTRAINT reservations_collection_fields_check CHECK (
        (state = 'waiting_for_archive' AND stream_id IS NOT NULL AND collection_job_id IS NULL)
        OR (state IN ('collecting', 'completed') AND stream_id IS NOT NULL AND collection_job_id IS NOT NULL)
        OR (
            state IN ('scheduled', 'monitoring', 'live', 'failed', 'canceled')
            AND collection_job_id IS NULL
        )
    ),
    CONSTRAINT reservations_terminal_timestamps_check CHECK (
        (state = 'completed') = (completed_at IS NOT NULL)
        AND (state = 'failed') = (failed_at IS NOT NULL)
        AND (state = 'canceled') = (canceled_at IS NOT NULL)
    ),
    CONSTRAINT reservations_monitoring_error_check CHECK (
        (
            last_error_code IS NULL
            AND last_error_message IS NULL
            AND last_error_retryable IS NULL
        )
        OR (
            last_error_code IS NOT NULL
            AND char_length(last_error_code) > 0
            AND last_error_message IS NOT NULL
            AND char_length(last_error_message) <= 1000
            AND last_error_retryable IS NOT NULL
        )
    ),
    CONSTRAINT reservations_lease_fields_check CHECK (
        (
            worker_id IS NULL
            AND lease_expires_at IS NULL
            AND heartbeat_at IS NULL
        )
        OR (
            worker_id IS NOT NULL
            AND char_length(worker_id) > 0
            AND lease_expires_at IS NOT NULL
            AND heartbeat_at IS NOT NULL
            AND lease_expires_at > heartbeat_at
            AND state IN ('scheduled', 'monitoring', 'live', 'waiting_for_archive', 'collecting')
        )
    )
);

CREATE UNIQUE INDEX reservations_active_video_uidx
    ON reservation.reservations (youtube_video_id)
    WHERE state NOT IN ('completed', 'failed', 'canceled');

CREATE UNIQUE INDEX reservations_collection_job_uidx
    ON reservation.reservations (collection_job_id)
    WHERE collection_job_id IS NOT NULL;

CREATE INDEX reservations_due_idx
    ON reservation.reservations (next_check_at, created_at, id)
    WHERE state IN ('scheduled', 'monitoring', 'live', 'waiting_for_archive', 'collecting');

CREATE INDEX reservations_stream_idx
    ON reservation.reservations (stream_id)
    WHERE stream_id IS NOT NULL;

CREATE TABLE reservation.reservation_transitions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reservation_id UUID NOT NULL REFERENCES reservation.reservations (id) ON DELETE CASCADE,
    from_state TEXT,
    to_state TEXT NOT NULL,
    reason_code TEXT NOT NULL,
    facts JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT reservation_transitions_from_state_check CHECK (
        from_state IS NULL
        OR from_state IN (
            'scheduled',
            'monitoring',
            'live',
            'waiting_for_archive',
            'collecting',
            'completed',
            'failed',
            'canceled'
        )
    ),
    CONSTRAINT reservation_transitions_to_state_check CHECK (
        to_state IN (
            'scheduled',
            'monitoring',
            'live',
            'waiting_for_archive',
            'collecting',
            'completed',
            'failed',
            'canceled'
        )
    ),
    CONSTRAINT reservation_transitions_state_change_check CHECK (
        from_state IS NULL OR from_state <> to_state
    ),
    CONSTRAINT reservation_transitions_reason_code_check CHECK (char_length(reason_code) > 0),
    CONSTRAINT reservation_transitions_facts_object_check CHECK (jsonb_typeof(facts) = 'object')
);

CREATE INDEX reservation_transitions_reservation_idx
    ON reservation.reservation_transitions (reservation_id, created_at, id);

ALTER TABLE collection.collection_jobs
    ADD COLUMN reservation_id UUID,
    ADD CONSTRAINT collection_jobs_reservation_fkey
        FOREIGN KEY (reservation_id) REFERENCES reservation.reservations (id);

CREATE UNIQUE INDEX collection_jobs_reservation_uidx
    ON collection.collection_jobs (reservation_id)
    WHERE reservation_id IS NOT NULL;

ALTER TABLE reservation.reservations
    ADD CONSTRAINT reservations_collection_job_fkey
        FOREIGN KEY (collection_job_id) REFERENCES collection.collection_jobs (id);
