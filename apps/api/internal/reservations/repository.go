package reservations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/streams"
)

const claimedReservationColumns = `
	r.id,
	r.youtube_video_id,
	r.source_url,
	r.state,
	r.scheduled_start_at,
	r.actual_start_at,
	r.actual_end_at,
	r.next_check_at,
	r.last_checked_at,
	r.monitor_attempt,
	r.last_error_code,
	r.last_error_message,
	r.last_error_retryable,
	r.stream_id,
	r.collection_job_id,
	r.worker_id,
	r.heartbeat_at,
	r.lease_expires_at,
	r.revision
`

const publicReservationColumns = `
	r.id,
	r.youtube_video_id,
	r.source_url,
	r.state,
	r.scheduled_start_at,
	r.actual_start_at,
	r.actual_end_at,
	r.next_check_at,
	r.last_checked_at,
	r.monitor_attempt,
	r.last_error_code,
	r.last_error_message,
	r.last_error_retryable,
	r.stream_id,
	r.collection_job_id,
	r.created_at,
	r.updated_at,
	c.status,
	c.error_code
`

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) Create(ctx context.Context, reservation Reservation) (Reservation, error) {
	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Reservation{}, fmt.Errorf("begin reservation creation: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()

	var id uuid.UUID
	err = transaction.QueryRow(ctx, `
		INSERT INTO reservation.reservations (
			youtube_video_id, source_url, state, next_check_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $4, $4)
		RETURNING id
	`, reservation.YouTubeVideoID, reservation.SourceURL, reservation.State, reservation.NextCheckAt).Scan(&id)
	if err != nil {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.ConstraintName == "reservations_active_video_uidx" {
			return Reservation{}, ErrAlreadyActive
		}
		return Reservation{}, fmt.Errorf("create reservation: %w", err)
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO reservation.reservation_transitions (
			reservation_id, from_state, to_state, reason_code, facts, created_at
		) VALUES ($1, NULL, $2, 'reservation_created', '{}'::jsonb, $3)
	`, id, reservation.State, reservation.NextCheckAt); err != nil {
		return Reservation{}, fmt.Errorf("record reservation creation: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return Reservation{}, fmt.Errorf("commit reservation creation: %w", err)
	}
	return repository.Get(ctx, id)
}

func (repository *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (Reservation, error) {
	reservation, err := scanReservation(repository.pool.QueryRow(ctx, `
		SELECT `+publicReservationColumns+`
		FROM reservation.reservations AS r
		LEFT JOIN collection.collection_jobs AS c ON c.id = r.collection_job_id
		WHERE r.id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Reservation{}, ErrNotFound
	}
	if err != nil {
		return Reservation{}, fmt.Errorf("get reservation: %w", err)
	}
	return reservation, nil
}

func (repository *PostgresRepository) List(ctx context.Context, options ListOptions) ([]Reservation, int, error) {
	var total int
	if err := repository.pool.QueryRow(ctx, "SELECT count(*) FROM reservation.reservations").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count reservations: %w", err)
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT `+publicReservationColumns+`
		FROM reservation.reservations AS r
		LEFT JOIN collection.collection_jobs AS c ON c.id = r.collection_job_id
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $1 OFFSET $2
	`, options.Limit, options.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list reservations: %w", err)
	}
	defer rows.Close()
	items := make([]Reservation, 0)
	for rows.Next() {
		reservation, err := scanReservation(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan reservation list: %w", err)
		}
		items = append(items, reservation)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate reservations: %w", err)
	}
	return items, total, nil
}

func (repository *PostgresRepository) Cancel(ctx context.Context, id uuid.UUID, canceledAt time.Time) (Reservation, error) {
	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Reservation{}, fmt.Errorf("begin reservation cancellation: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()
	var state State
	if err := transaction.QueryRow(ctx, `
		SELECT state FROM reservation.reservations WHERE id = $1 FOR UPDATE
	`, id).Scan(&state); errors.Is(err, pgx.ErrNoRows) {
		return Reservation{}, ErrNotFound
	} else if err != nil {
		return Reservation{}, fmt.Errorf("lock reservation for cancellation: %w", err)
	}
	if !(Reservation{State: state}).CanCancel() {
		return Reservation{}, ErrNotCancellable
	}
	if _, err := transaction.Exec(ctx, `
		UPDATE reservation.reservations
		SET state = 'canceled', canceled_at = $2, next_check_at = $2,
		    worker_id = NULL, heartbeat_at = NULL, lease_expires_at = NULL,
		    revision = revision + 1, updated_at = $2
		WHERE id = $1
	`, id, canceledAt); err != nil {
		return Reservation{}, fmt.Errorf("cancel reservation: %w", err)
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO reservation.reservation_transitions (
			reservation_id, from_state, to_state, reason_code, facts, created_at
		) VALUES ($1, $2, 'canceled', 'user_canceled', '{}'::jsonb, $3)
	`, id, state, canceledAt); err != nil {
		return Reservation{}, fmt.Errorf("record reservation cancellation: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return Reservation{}, fmt.Errorf("commit reservation cancellation: %w", err)
	}
	return repository.Get(ctx, id)
}

func (repository *PostgresRepository) ClaimDue(
	ctx context.Context,
	workerID string,
	claimedAt time.Time,
	leaseDuration time.Duration,
) (*ClaimedReservation, error) {
	if workerID == "" {
		return nil, errors.New("reservation worker ID is required")
	}
	if leaseDuration <= 0 {
		return nil, errors.New("reservation lease duration must be positive")
	}
	expiresAt := claimedAt.Add(leaseDuration)
	claimed, err := scanClaimedReservation(repository.pool.QueryRow(ctx, `
		WITH candidate AS (
			SELECT id
			FROM reservation.reservations
			WHERE state IN ('scheduled', 'monitoring', 'live', 'waiting_for_archive', 'collecting')
			  AND next_check_at <= $2
			  AND (lease_expires_at IS NULL OR lease_expires_at <= $2)
			ORDER BY next_check_at, created_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE reservation.reservations AS r
		SET worker_id = $1,
		    heartbeat_at = $2,
		    lease_expires_at = $3,
		    revision = r.revision + 1,
		    updated_at = $2
		FROM candidate
		WHERE r.id = candidate.id
			RETURNING `+claimedReservationColumns,
		workerID, claimedAt, expiresAt))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claim due reservation: %w", err)
	}
	return &claimed, nil
}

func (repository *PostgresRepository) Heartbeat(
	ctx context.Context,
	claimed ClaimedReservation,
	heartbeatAt time.Time,
	leaseDuration time.Duration,
) (ClaimedReservation, error) {
	if leaseDuration <= 0 {
		return ClaimedReservation{}, errors.New("reservation lease duration must be positive")
	}
	expiresAt := heartbeatAt.Add(leaseDuration)
	updated, err := scanClaimedReservation(repository.pool.QueryRow(ctx, `
		UPDATE reservation.reservations AS r
		SET heartbeat_at = $4,
		    lease_expires_at = $5,
		    revision = r.revision + 1,
		    updated_at = $4
		WHERE r.id = $1
		  AND r.worker_id = $2
		  AND r.revision = $3
		  AND r.lease_expires_at > $4
		  AND r.state IN ('scheduled', 'monitoring', 'live', 'waiting_for_archive', 'collecting')
			RETURNING `+claimedReservationColumns,
		claimed.ID, claimed.Lease.WorkerID, claimed.Lease.Revision, heartbeatAt, expiresAt))
	if errors.Is(err, pgx.ErrNoRows) {
		return ClaimedReservation{}, ErrLeaseLost
	}
	if err != nil {
		return ClaimedReservation{}, fmt.Errorf("heartbeat reservation: %w", err)
	}
	return updated, nil
}

func (repository *PostgresRepository) ApplyMetadata(
	ctx context.Context,
	claimed ClaimedReservation,
	metadata streams.Metadata,
	checkedAt time.Time,
) error {
	targetState, reasonCode, err := stateFromMetadata(claimed.State, metadata)
	if err != nil {
		return err
	}
	nextCheckAt := nextCheck(targetState, metadata.ScheduledStartAt, claimed.MonitorAttempt+1, checkedAt)

	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin reservation metadata update: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()

	var streamID *uuid.UUID
	if metadata.LifecycleStatus == streams.LifecycleEnded {
		id, err := upsertObservedStream(ctx, transaction, metadata)
		if err != nil {
			return err
		}
		streamID = &id
	}

	var collectionJobID *uuid.UUID
	if targetState == StateCollecting {
		if streamID == nil {
			return errors.New("archive-ready reservation requires a stream")
		}
		id, err := ensureAutomaticCollectionJob(ctx, transaction, claimed.ID, *streamID)
		if err != nil {
			return err
		}
		collectionJobID = &id
	}

	command, err := transaction.Exec(ctx, `
		UPDATE reservation.reservations
		SET state = $4,
		    scheduled_start_at = COALESCE($5, scheduled_start_at),
		    actual_start_at = COALESCE($6, actual_start_at),
		    actual_end_at = COALESCE($7, actual_end_at),
		    next_check_at = $8,
		    last_checked_at = $9,
		    monitor_attempt = monitor_attempt + 1,
		    last_error_code = NULL,
		    last_error_message = NULL,
		    last_error_retryable = NULL,
		    stream_id = COALESCE($10, stream_id),
		    collection_job_id = $11,
		    worker_id = NULL,
		    lease_expires_at = NULL,
		    heartbeat_at = NULL,
		    revision = revision + 1,
		    updated_at = $9
		WHERE id = $1
		  AND worker_id = $2
		  AND revision = $3
		  AND lease_expires_at > $9
	`,
		claimed.ID,
		claimed.Lease.WorkerID,
		claimed.Lease.Revision,
		targetState,
		metadata.ScheduledStartAt,
		metadata.ActualStartAt,
		metadata.ActualEndAt,
		nextCheckAt,
		checkedAt,
		streamID,
		collectionJobID,
	)
	if err != nil {
		return fmt.Errorf("update reservation metadata: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrLeaseLost
	}
	if targetState != claimed.State {
		if _, err := transaction.Exec(ctx, `
			INSERT INTO reservation.reservation_transitions (
				reservation_id, from_state, to_state, reason_code, facts, created_at
			) VALUES ($1, $2, $3, $4, '{}'::jsonb, $5)
		`, claimed.ID, claimed.State, targetState, reasonCode, checkedAt); err != nil {
			return fmt.Errorf("record reservation metadata transition: %w", err)
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit reservation metadata update: %w", err)
	}
	return nil
}

func (repository *PostgresRepository) SyncCollection(
	ctx context.Context,
	claimed ClaimedReservation,
	checkedAt time.Time,
) error {
	if claimed.CollectionJobID == nil {
		return errors.New("collecting reservation has no collection job")
	}
	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin reservation collection sync: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()

	var jobStatus string
	if err := transaction.QueryRow(ctx, `
		SELECT status FROM collection.collection_jobs WHERE id = $1 AND reservation_id = $2
	`, *claimed.CollectionJobID, claimed.ID).Scan(&jobStatus); err != nil {
		return fmt.Errorf("read reservation collection job: %w", err)
	}
	targetState := StateCollecting
	var completedAt *time.Time
	if jobStatus == "succeeded" {
		targetState = StateCompleted
		completedAt = &checkedAt
	}
	command, err := transaction.Exec(ctx, `
		UPDATE reservation.reservations
		SET state = $4,
		    next_check_at = $5,
		    last_checked_at = $6,
		    completed_at = $7,
		    worker_id = NULL,
		    lease_expires_at = NULL,
		    heartbeat_at = NULL,
		    revision = revision + 1,
		    updated_at = $6
		WHERE id = $1
		  AND worker_id = $2
		  AND revision = $3
		  AND lease_expires_at > $6
	`,
		claimed.ID,
		claimed.Lease.WorkerID,
		claimed.Lease.Revision,
		targetState,
		checkedAt.Add(time.Minute),
		checkedAt,
		completedAt,
	)
	if err != nil {
		return fmt.Errorf("sync reservation collection: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrLeaseLost
	}
	if targetState == StateCompleted {
		if _, err := transaction.Exec(ctx, `
			INSERT INTO reservation.reservation_transitions (
				reservation_id, from_state, to_state, reason_code, facts, created_at
			) VALUES ($1, $2, $3, 'collection_succeeded', '{}'::jsonb, $4)
		`, claimed.ID, claimed.State, targetState, checkedAt); err != nil {
			return fmt.Errorf("record reservation completion: %w", err)
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit reservation collection sync: %w", err)
	}
	return nil
}

func (repository *PostgresRepository) RecordFailure(
	ctx context.Context,
	claimed ClaimedReservation,
	checkedAt time.Time,
	code string,
	message string,
	retryable bool,
) error {
	attempt := claimed.MonitorAttempt + 1
	targetState := claimed.State
	nextCheckAt := checkedAt.Add(boundedBackoff(attempt, time.Minute, 30*time.Minute))
	var failedAt *time.Time
	if !retryable || attempt >= MaxMonitorAttempts {
		targetState = StateFailed
		retryable = false
		nextCheckAt = checkedAt
		failedAt = &checkedAt
	}

	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin reservation failure update: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()
	command, err := transaction.Exec(ctx, `
		UPDATE reservation.reservations
		SET state = $4,
		    next_check_at = $5,
		    last_checked_at = $6,
		    monitor_attempt = monitor_attempt + 1,
		    last_error_code = $7,
		    last_error_message = $8,
		    last_error_retryable = $9,
		    failed_at = $10,
		    worker_id = NULL,
		    lease_expires_at = NULL,
		    heartbeat_at = NULL,
		    revision = revision + 1,
		    updated_at = $6
		WHERE id = $1
		  AND worker_id = $2
		  AND revision = $3
		  AND lease_expires_at > $6
	`,
		claimed.ID,
		claimed.Lease.WorkerID,
		claimed.Lease.Revision,
		targetState,
		nextCheckAt,
		checkedAt,
		code,
		message,
		retryable,
		failedAt,
	)
	if err != nil {
		return fmt.Errorf("record reservation failure: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrLeaseLost
	}
	if targetState == StateFailed {
		reasonCode := "permanent_monitoring_failure"
		if attempt >= MaxMonitorAttempts {
			reasonCode = "monitoring_retries_exhausted"
		}
		if _, err := transaction.Exec(ctx, `
			INSERT INTO reservation.reservation_transitions (
				reservation_id, from_state, to_state, reason_code, facts, created_at
			) VALUES ($1, $2, $3, $4, '{}'::jsonb, $5)
		`, claimed.ID, claimed.State, targetState, reasonCode, checkedAt); err != nil {
			return fmt.Errorf("record reservation failure transition: %w", err)
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit reservation failure update: %w", err)
	}
	return nil
}

func stateFromMetadata(current State, metadata streams.Metadata) (State, string, error) {
	switch metadata.LifecycleStatus {
	case streams.LifecycleScheduled:
		if current == StateScheduled || current == StateMonitoring {
			return current, "broadcast_scheduled", nil
		}
	case streams.LifecycleLive:
		if current == StateScheduled || current == StateMonitoring {
			next, err := Transition(current, EventBroadcastStarted)
			return next, "broadcast_started", err
		}
	case streams.LifecycleEnded:
		if current == StateWaitingForArchive {
			if metadata.Duration == nil {
				return current, "archive_processing", nil
			}
			next, err := Transition(current, EventArchiveReady)
			return next, "archive_ready", err
		}
		if current == StateScheduled || current == StateMonitoring || current == StateLive {
			next, err := Transition(current, EventBroadcastEnded)
			return next, "broadcast_ended", err
		}
	default:
		if current == StateScheduled {
			next, err := Transition(current, EventMonitor)
			return next, "broadcast_unknown", err
		}
	}
	return current, "observation_unchanged", nil
}

func nextCheck(state State, scheduledStart *time.Time, attempt int, checkedAt time.Time) time.Time {
	switch state {
	case StateScheduled:
		if scheduledStart != nil {
			if !scheduledStart.After(checkedAt.Add(15 * time.Minute)) {
				return checkedAt.Add(time.Minute)
			}
			candidate := scheduledStart.Add(-15 * time.Minute)
			if candidate.Before(checkedAt.Add(15 * time.Minute)) {
				return candidate
			}
		}
		return checkedAt.Add(15 * time.Minute)
	case StateLive:
		return checkedAt.Add(2 * time.Minute)
	case StateWaitingForArchive:
		return checkedAt.Add(boundedBackoff(attempt, 2*time.Minute, 30*time.Minute))
	case StateCollecting:
		return checkedAt.Add(time.Minute)
	default:
		return checkedAt.Add(time.Minute)
	}
}

func boundedBackoff(attempt int, initial, maximum time.Duration) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := initial
	for range attempt - 1 {
		if delay >= maximum/2 {
			return maximum
		}
		delay *= 2
	}
	return delay
}

func upsertObservedStream(ctx context.Context, transaction pgx.Tx, metadata streams.Metadata) (uuid.UUID, error) {
	var durationMilliseconds *int64
	if metadata.Duration != nil {
		value := metadata.Duration.Milliseconds()
		durationMilliseconds = &value
	}
	var id uuid.UUID
	err := transaction.QueryRow(ctx, `
		INSERT INTO stream.streams (
			youtube_video_id, canonical_url, title, channel_id, channel_title, thumbnail_url,
			scheduled_start_at, actual_start_at, actual_end_at, duration_ms, lifecycle_status,
			metadata_fetched_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (youtube_video_id) DO UPDATE SET
			canonical_url = EXCLUDED.canonical_url,
			title = EXCLUDED.title,
			channel_id = EXCLUDED.channel_id,
			channel_title = EXCLUDED.channel_title,
			thumbnail_url = EXCLUDED.thumbnail_url,
			scheduled_start_at = EXCLUDED.scheduled_start_at,
			actual_start_at = EXCLUDED.actual_start_at,
			actual_end_at = EXCLUDED.actual_end_at,
			duration_ms = EXCLUDED.duration_ms,
			lifecycle_status = EXCLUDED.lifecycle_status,
			metadata_fetched_at = EXCLUDED.metadata_fetched_at,
			updated_at = $12
		RETURNING id
	`,
		metadata.YouTubeVideoID,
		metadata.CanonicalURL,
		metadata.Title,
		metadata.ChannelID,
		metadata.ChannelTitle,
		metadata.ThumbnailURL,
		metadata.ScheduledStartAt,
		metadata.ActualStartAt,
		metadata.ActualEndAt,
		durationMilliseconds,
		metadata.LifecycleStatus,
		metadata.MetadataFetchedAt,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("upsert reservation stream: %w", err)
	}
	return id, nil
}

func ensureAutomaticCollectionJob(
	ctx context.Context,
	transaction pgx.Tx,
	reservationID uuid.UUID,
	streamID uuid.UUID,
) (uuid.UUID, error) {
	var id uuid.UUID
	err := transaction.QueryRow(ctx, `
		SELECT id
		FROM collection.collection_jobs
		WHERE reservation_id = $1
		FOR UPDATE
	`, reservationID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("find reservation collection job: %w", err)
	}

	err = transaction.QueryRow(ctx, `
		SELECT id
		FROM collection.collection_jobs
		WHERE stream_id = $1
		  AND kind = 'chat_replay'
		  AND status IN ('queued', 'running')
		ORDER BY requested_at, id
		FOR UPDATE
		LIMIT 1
	`, streamID).Scan(&id)
	if err == nil {
		command, err := transaction.Exec(ctx, `
			UPDATE collection.collection_jobs
			SET reservation_id = $2, updated_at = CURRENT_TIMESTAMP
			WHERE id = $1 AND reservation_id IS NULL
		`, id, reservationID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("link active collection job to reservation: %w", err)
		}
		if command.RowsAffected() != 1 {
			return uuid.Nil, errors.New("active collection job belongs to another reservation")
		}
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("find active collection job: %w", err)
	}

	err = transaction.QueryRow(ctx, `
		INSERT INTO collection.collection_jobs (stream_id, reservation_id, kind)
		VALUES ($1, $2, 'chat_replay')
		RETURNING id
	`, streamID, reservationID).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("ensure automatic collection job: %w", err)
	}
	return id, nil
}

type reservationRow interface {
	Scan(...any) error
}

func scanReservation(row reservationRow) (Reservation, error) {
	var (
		reservation        Reservation
		state              string
		scheduledStart     pgtype.Timestamptz
		actualStart        pgtype.Timestamptz
		actualEnd          pgtype.Timestamptz
		lastChecked        pgtype.Timestamptz
		lastErrorCode      pgtype.Text
		lastErrorMessage   pgtype.Text
		lastErrorRetryable pgtype.Bool
		streamID           pgtype.UUID
		collectionJobID    pgtype.UUID
		collectionStatus   pgtype.Text
		collectionError    pgtype.Text
	)
	err := row.Scan(
		&reservation.ID,
		&reservation.YouTubeVideoID,
		&reservation.SourceURL,
		&state,
		&scheduledStart,
		&actualStart,
		&actualEnd,
		&reservation.NextCheckAt,
		&lastChecked,
		&reservation.MonitorAttempt,
		&lastErrorCode,
		&lastErrorMessage,
		&lastErrorRetryable,
		&streamID,
		&collectionJobID,
		&reservation.CreatedAt,
		&reservation.UpdatedAt,
		&collectionStatus,
		&collectionError,
	)
	if err != nil {
		return Reservation{}, err
	}
	reservation.State = State(state)
	reservation.ScheduledStartAt = timestampPointer(scheduledStart)
	reservation.ActualStartAt = timestampPointer(actualStart)
	reservation.ActualEndAt = timestampPointer(actualEnd)
	reservation.LastCheckedAt = timestampPointer(lastChecked)
	reservation.LastErrorCode = textPointer(lastErrorCode)
	reservation.LastErrorMessage = textPointer(lastErrorMessage)
	reservation.LastErrorRetryable = boolPointer(lastErrorRetryable)
	reservation.StreamID = uuidPointer(streamID)
	reservation.CollectionJobID = uuidPointer(collectionJobID)
	reservation.CollectionStatus = textPointer(collectionStatus)
	reservation.CollectionErrorCode = textPointer(collectionError)
	return reservation, nil
}

func scanClaimedReservation(row reservationRow) (ClaimedReservation, error) {
	var (
		claimed            ClaimedReservation
		state              string
		scheduledStart     pgtype.Timestamptz
		actualStart        pgtype.Timestamptz
		actualEnd          pgtype.Timestamptz
		lastChecked        pgtype.Timestamptz
		lastErrorCode      pgtype.Text
		lastErrorMessage   pgtype.Text
		lastErrorRetryable pgtype.Bool
		streamID           pgtype.UUID
		collectionJobID    pgtype.UUID
		workerID           pgtype.Text
		heartbeatAt        pgtype.Timestamptz
		leaseExpiresAt     pgtype.Timestamptz
	)
	err := row.Scan(
		&claimed.ID,
		&claimed.YouTubeVideoID,
		&claimed.SourceURL,
		&state,
		&scheduledStart,
		&actualStart,
		&actualEnd,
		&claimed.NextCheckAt,
		&lastChecked,
		&claimed.MonitorAttempt,
		&lastErrorCode,
		&lastErrorMessage,
		&lastErrorRetryable,
		&streamID,
		&collectionJobID,
		&workerID,
		&heartbeatAt,
		&leaseExpiresAt,
		&claimed.Lease.Revision,
	)
	if err != nil {
		return ClaimedReservation{}, err
	}
	claimed.State = State(state)
	claimed.ScheduledStartAt = timestampPointer(scheduledStart)
	claimed.ActualStartAt = timestampPointer(actualStart)
	claimed.ActualEndAt = timestampPointer(actualEnd)
	claimed.LastCheckedAt = timestampPointer(lastChecked)
	claimed.LastErrorCode = textPointer(lastErrorCode)
	claimed.LastErrorMessage = textPointer(lastErrorMessage)
	claimed.LastErrorRetryable = boolPointer(lastErrorRetryable)
	claimed.StreamID = uuidPointer(streamID)
	claimed.CollectionJobID = uuidPointer(collectionJobID)
	if workerID.Valid {
		claimed.Lease.WorkerID = workerID.String
	}
	if heartbeatAt.Valid {
		claimed.Lease.HeartbeatAt = heartbeatAt.Time
	}
	if leaseExpiresAt.Valid {
		claimed.Lease.ExpiresAt = leaseExpiresAt.Time
	}
	return claimed, nil
}

func timestampPointer(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func textPointer(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func boolPointer(value pgtype.Bool) *bool {
	if !value.Valid {
		return nil
	}
	return &value.Bool
}

func uuidPointer(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	id := uuid.UUID(value.Bytes)
	return &id
}
