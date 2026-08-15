package collections

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const jobColumns = `
	id,
	stream_id,
	kind,
	status,
	attempt,
	processed_count,
	skipped_count,
	requested_at,
	started_at,
	updated_at,
	finished_at,
	error_code
`

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) Start(ctx context.Context, streamID uuid.UUID) (Job, error) {
	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Job{}, fmt.Errorf("begin collection start: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()

	if err := lockStream(ctx, transaction, streamID); err != nil {
		return Job{}, err
	}
	job, err := activeJob(ctx, transaction, streamID)
	if err == nil {
		if err := transaction.Commit(ctx); err != nil {
			return Job{}, fmt.Errorf("commit idempotent collection start: %w", err)
		}
		return job, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Job{}, fmt.Errorf("find active collection job: %w", err)
	}

	job, err = insertJob(ctx, transaction, streamID)
	if err != nil {
		return Job{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return Job{}, fmt.Errorf("commit collection start: %w", err)
	}
	return job, nil
}

func (repository *PostgresRepository) Latest(ctx context.Context, streamID uuid.UUID) (Job, error) {
	var streamExists bool
	if err := repository.pool.QueryRow(
		ctx,
		"SELECT EXISTS (SELECT 1 FROM stream.streams WHERE id = $1)",
		streamID,
	).Scan(&streamExists); err != nil {
		return Job{}, fmt.Errorf("check stream for latest collection: %w", err)
	}
	if !streamExists {
		return Job{}, ErrStreamNotFound
	}

	job, err := scanJob(repository.pool.QueryRow(ctx, `
		SELECT `+jobColumns+`
		FROM collection.collection_jobs
		WHERE stream_id = $1 AND kind = 'chat_replay'
		ORDER BY requested_at DESC, id DESC
		LIMIT 1
	`, streamID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrJobNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("get latest collection job: %w", err)
	}
	return job, nil
}

func (repository *PostgresRepository) Retry(ctx context.Context, jobID uuid.UUID) (Job, error) {
	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Job{}, fmt.Errorf("begin collection retry: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()

	var streamID uuid.UUID
	if err := transaction.QueryRow(
		ctx,
		"SELECT stream_id FROM collection.collection_jobs WHERE id = $1",
		jobID,
	).Scan(&streamID); errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrJobNotFound
	} else if err != nil {
		return Job{}, fmt.Errorf("find collection job for retry: %w", err)
	}
	if err := lockStream(ctx, transaction, streamID); err != nil {
		return Job{}, err
	}

	failed, err := scanJob(transaction.QueryRow(ctx, `
		SELECT `+jobColumns+`
		FROM collection.collection_jobs
		WHERE id = $1
		FOR UPDATE
	`, jobID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrJobNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("lock collection job for retry: %w", err)
	}
	if failed.Status != StatusFailed || failed.Error == nil || !failed.Error.Retryable {
		return Job{}, ErrNotRetryable
	}
	if _, err := activeJob(ctx, transaction, streamID); err == nil {
		return Job{}, ErrActiveJob
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Job{}, fmt.Errorf("find active collection job before retry: %w", err)
	}

	job, err := insertJob(ctx, transaction, streamID)
	if err != nil {
		return Job{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return Job{}, fmt.Errorf("commit collection retry: %w", err)
	}
	return job, nil
}

func (repository *PostgresRepository) ListMessages(
	ctx context.Context,
	streamID uuid.UUID,
	limit int,
	cursor *Cursor,
) ([]ChatMessage, error) {
	var streamExists bool
	if err := repository.pool.QueryRow(
		ctx,
		"SELECT EXISTS (SELECT 1 FROM stream.streams WHERE id = $1)",
		streamID,
	).Scan(&streamExists); err != nil {
		return nil, fmt.Errorf("check stream for chat list: %w", err)
	}
	if !streamExists {
		return nil, ErrStreamNotFound
	}

	offset := int64(-1 << 63)
	id := uuid.Nil
	if cursor != nil {
		offset = cursor.OffsetMilliseconds
		id = cursor.ID
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT id, author_channel_id, author_display_name, message_text,
		       published_at, offset_milliseconds, message_type
		FROM chat.chat_messages
		WHERE stream_id = $1
		  AND (offset_milliseconds, id) > ($2, $3)
		ORDER BY offset_milliseconds, id
		LIMIT $4
	`, streamID, offset, id, limit)
	if err != nil {
		return nil, fmt.Errorf("list chat messages: %w", err)
	}
	defer rows.Close()

	items := make([]ChatMessage, 0, limit)
	for rows.Next() {
		var item ChatMessage
		var authorChannelID pgtype.Text
		if err := rows.Scan(
			&item.ID,
			&authorChannelID,
			&item.AuthorDisplayName,
			&item.MessageText,
			&item.PublishedAt,
			&item.OffsetMilliseconds,
			&item.MessageType,
		); err != nil {
			return nil, fmt.Errorf("scan chat message: %w", err)
		}
		if authorChannelID.Valid {
			item.AuthorChannelID = &authorChannelID.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat messages: %w", err)
	}
	return items, nil
}

func lockStream(ctx context.Context, transaction pgx.Tx, streamID uuid.UUID) error {
	var id uuid.UUID
	err := transaction.QueryRow(
		ctx,
		"SELECT id FROM stream.streams WHERE id = $1 FOR UPDATE",
		streamID,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrStreamNotFound
	}
	if err != nil {
		return fmt.Errorf("lock stream for collection: %w", err)
	}
	return nil
}

func activeJob(ctx context.Context, transaction pgx.Tx, streamID uuid.UUID) (Job, error) {
	return scanJob(transaction.QueryRow(ctx, `
		SELECT `+jobColumns+`
		FROM collection.collection_jobs
		WHERE stream_id = $1
		  AND kind = 'chat_replay'
		  AND status IN ('queued', 'running')
		ORDER BY requested_at DESC, id DESC
		LIMIT 1
	`, streamID))
}

func insertJob(ctx context.Context, transaction pgx.Tx, streamID uuid.UUID) (Job, error) {
	job, err := scanJob(transaction.QueryRow(ctx, `
		INSERT INTO collection.collection_jobs (stream_id, kind)
		VALUES ($1, 'chat_replay')
		RETURNING `+jobColumns,
		streamID,
	))
	if err != nil {
		return Job{}, fmt.Errorf("insert collection job: %w", err)
	}
	return job, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanJob(row rowScanner) (Job, error) {
	var job Job
	var status string
	var startedAt pgtype.Timestamptz
	var finishedAt pgtype.Timestamptz
	var errorCode pgtype.Text
	if err := row.Scan(
		&job.ID,
		&job.StreamID,
		&job.Kind,
		&status,
		&job.Attempt,
		&job.ProcessedCount,
		&job.SkippedCount,
		&job.RequestedAt,
		&startedAt,
		&job.UpdatedAt,
		&finishedAt,
		&errorCode,
	); err != nil {
		return Job{}, err
	}
	job.Status = JobStatus(status)
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		job.FinishedAt = &finishedAt.Time
	}
	if errorCode.Valid {
		job.Error = safeErrorFor(errorCode.String)
	}
	return job, nil
}
