package streams

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
)

const streamColumns = `
	id,
	youtube_video_id,
	canonical_url,
	title,
	channel_id,
	channel_title,
	thumbnail_url,
	scheduled_start_at,
	actual_start_at,
	actual_end_at,
	duration_ms,
	lifecycle_status,
	metadata_fetched_at,
	created_at,
	updated_at
`

type Repository interface {
	Create(context.Context, Metadata) (Stream, error)
	Upsert(context.Context, Metadata) (Stream, error)
	Get(context.Context, uuid.UUID) (Stream, error)
	GetByYouTubeVideoID(context.Context, string) (Stream, error)
	List(context.Context, ListOptions) ([]Stream, error)
}

type ListItemRepository interface {
	ListItems(context.Context, ListOptions) ([]ListItem, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

var _ Repository = (*PostgresRepository)(nil)
var _ ListItemRepository = (*PostgresRepository)(nil)

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) Create(ctx context.Context, metadata Metadata) (Stream, error) {
	metadata, err := normalizeMetadata(metadata)
	if err != nil {
		return Stream{}, err
	}

	stream, err := scanStream(repository.pool.QueryRow(ctx, `
		INSERT INTO stream.streams (
			youtube_video_id, canonical_url, title, channel_id, channel_title, thumbnail_url,
			scheduled_start_at, actual_start_at, actual_end_at, duration_ms, lifecycle_status,
			metadata_fetched_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING `+streamColumns,
		metadata.YouTubeVideoID,
		metadata.CanonicalURL,
		metadata.Title,
		metadata.ChannelID,
		metadata.ChannelTitle,
		metadata.ThumbnailURL,
		metadata.ScheduledStartAt,
		metadata.ActualStartAt,
		metadata.ActualEndAt,
		durationMilliseconds(metadata.Duration),
		metadata.LifecycleStatus,
		metadata.MetadataFetchedAt,
	))
	if isYouTubeVideoIDConflict(err) {
		return Stream{}, ErrYouTubeVideoIDExists
	}
	if err != nil {
		return Stream{}, fmt.Errorf("create stream: %w", err)
	}
	return stream, nil
}

func (repository *PostgresRepository) Upsert(ctx context.Context, metadata Metadata) (Stream, error) {
	metadata, err := normalizeMetadata(metadata)
	if err != nil {
		return Stream{}, err
	}

	stream, err := scanStream(repository.pool.QueryRow(ctx, `
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
			updated_at = CURRENT_TIMESTAMP
		RETURNING `+streamColumns,
		metadata.YouTubeVideoID,
		metadata.CanonicalURL,
		metadata.Title,
		metadata.ChannelID,
		metadata.ChannelTitle,
		metadata.ThumbnailURL,
		metadata.ScheduledStartAt,
		metadata.ActualStartAt,
		metadata.ActualEndAt,
		durationMilliseconds(metadata.Duration),
		metadata.LifecycleStatus,
		metadata.MetadataFetchedAt,
	))
	if err != nil {
		return Stream{}, fmt.Errorf("upsert stream: %w", err)
	}
	return stream, nil
}

func (repository *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (Stream, error) {
	stream, err := scanStream(repository.pool.QueryRow(
		ctx,
		"SELECT "+streamColumns+" FROM stream.streams WHERE id = $1",
		id,
	))
	return mapReadResult("get stream", stream, err)
}

func (repository *PostgresRepository) GetByYouTubeVideoID(
	ctx context.Context,
	youTubeVideoID string,
) (Stream, error) {
	stream, err := scanStream(repository.pool.QueryRow(
		ctx,
		"SELECT "+streamColumns+" FROM stream.streams WHERE youtube_video_id = $1",
		youTubeVideoID,
	))
	return mapReadResult("get stream by YouTube video ID", stream, err)
}

func (repository *PostgresRepository) List(
	ctx context.Context,
	options ListOptions,
) ([]Stream, error) {
	if err := validateListOptions(options); err != nil {
		return nil, err
	}

	rows, err := repository.pool.Query(ctx, `
		SELECT `+streamColumns+`
		FROM stream.streams
		ORDER BY created_at DESC, id DESC
		LIMIT $1 OFFSET $2
	`, options.Limit, options.Offset)
	if err != nil {
		return nil, fmt.Errorf("list streams: %w", err)
	}
	defer rows.Close()

	streams := make([]Stream, 0, options.Limit)
	for rows.Next() {
		stream, err := scanStream(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed stream: %w", err)
		}
		streams = append(streams, stream)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate streams: %w", err)
	}
	return streams, nil
}

func (repository *PostgresRepository) ListItems(
	ctx context.Context,
	options ListOptions,
) ([]ListItem, error) {
	if err := validateListOptions(options); err != nil {
		return nil, err
	}

	rows, err := repository.pool.Query(ctx, `
		SELECT `+streamColumns+`,
			(
				SELECT status
				FROM collection.collection_jobs
				WHERE collection.collection_jobs.stream_id = stream.streams.id
				ORDER BY requested_at DESC, id DESC
				LIMIT 1
			) AS collection_status,
			(
				SELECT COUNT(*)
				FROM chat.chat_messages
				WHERE chat.chat_messages.stream_id = stream.streams.id
			) AS chat_message_count
		FROM stream.streams
		ORDER BY created_at DESC, id DESC
		LIMIT $1 OFFSET $2
	`, options.Limit, options.Offset)
	if err != nil {
		return nil, fmt.Errorf("list stream read models: %w", err)
	}
	defer rows.Close()

	items := make([]ListItem, 0, options.Limit)
	for rows.Next() {
		item, err := scanListItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan stream list item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stream list items: %w", err)
	}
	return items, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanStream(row rowScanner) (Stream, error) {
	var (
		stream         Stream
		thumbnailURL   pgtype.Text
		scheduledStart pgtype.Timestamptz
		actualStart    pgtype.Timestamptz
		actualEnd      pgtype.Timestamptz
		duration       pgtype.Int8
		status         string
	)

	err := row.Scan(
		&stream.ID,
		&stream.YouTubeVideoID,
		&stream.CanonicalURL,
		&stream.Title,
		&stream.ChannelID,
		&stream.ChannelTitle,
		&thumbnailURL,
		&scheduledStart,
		&actualStart,
		&actualEnd,
		&duration,
		&status,
		&stream.MetadataFetchedAt,
		&stream.CreatedAt,
		&stream.UpdatedAt,
	)
	if err != nil {
		return Stream{}, err
	}

	applyNullableStreamFields(
		&stream,
		thumbnailURL,
		scheduledStart,
		actualStart,
		actualEnd,
		duration,
		status,
	)
	return stream, nil
}

func scanListItem(row rowScanner) (ListItem, error) {
	var (
		item             ListItem
		thumbnailURL     pgtype.Text
		scheduledStart   pgtype.Timestamptz
		actualStart      pgtype.Timestamptz
		actualEnd        pgtype.Timestamptz
		duration         pgtype.Int8
		lifecycleStatus  string
		collectionStatus pgtype.Text
	)

	err := row.Scan(
		&item.ID,
		&item.YouTubeVideoID,
		&item.CanonicalURL,
		&item.Title,
		&item.ChannelID,
		&item.ChannelTitle,
		&thumbnailURL,
		&scheduledStart,
		&actualStart,
		&actualEnd,
		&duration,
		&lifecycleStatus,
		&item.MetadataFetchedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
		&collectionStatus,
		&item.ChatMessageCount,
	)
	if err != nil {
		return ListItem{}, err
	}

	applyNullableStreamFields(
		&item.Stream,
		thumbnailURL,
		scheduledStart,
		actualStart,
		actualEnd,
		duration,
		lifecycleStatus,
	)
	item.CollectionStatus = textPointer(collectionStatus)
	return item, nil
}

func applyNullableStreamFields(
	stream *Stream,
	thumbnailURL pgtype.Text,
	scheduledStart pgtype.Timestamptz,
	actualStart pgtype.Timestamptz,
	actualEnd pgtype.Timestamptz,
	duration pgtype.Int8,
	status string,
) {
	stream.ThumbnailURL = textPointer(thumbnailURL)
	stream.ScheduledStartAt = timePointer(scheduledStart)
	stream.ActualStartAt = timePointer(actualStart)
	stream.ActualEndAt = timePointer(actualEnd)
	stream.Duration = durationPointer(duration)
	stream.LifecycleStatus = LifecycleStatus(status)
}

func validateListOptions(options ListOptions) error {
	if options.Limit < 1 || options.Limit > 100 {
		return fmt.Errorf("%w: list limit must be between 1 and 100", ErrInvalidStream)
	}
	if options.Offset < 0 {
		return fmt.Errorf("%w: list offset must not be negative", ErrInvalidStream)
	}
	return nil
}

func normalizeMetadata(metadata Metadata) (Metadata, error) {
	if metadata.YouTubeVideoID == "" || len(metadata.YouTubeVideoID) > 64 {
		return Metadata{}, fmt.Errorf("%w: YouTube video ID is required and must not exceed 64 characters", ErrInvalidStream)
	}
	if metadata.CanonicalURL == "" || metadata.Title == "" || metadata.ChannelID == "" || metadata.ChannelTitle == "" {
		return Metadata{}, fmt.Errorf("%w: canonical URL, title, channel ID, and channel title are required", ErrInvalidStream)
	}
	if metadata.MetadataFetchedAt.IsZero() {
		return Metadata{}, fmt.Errorf("%w: metadata fetched time is required", ErrInvalidStream)
	}
	if metadata.Duration != nil && *metadata.Duration < 0 {
		return Metadata{}, fmt.Errorf("%w: duration must not be negative", ErrInvalidStream)
	}
	if metadata.ActualStartAt != nil && metadata.ActualEndAt != nil && metadata.ActualEndAt.Before(*metadata.ActualStartAt) {
		return Metadata{}, fmt.Errorf("%w: actual end must not precede actual start", ErrInvalidStream)
	}
	if metadata.LifecycleStatus == "" {
		metadata.LifecycleStatus = LifecycleUnknown
	}
	if !metadata.LifecycleStatus.Valid() {
		return Metadata{}, fmt.Errorf("%w: unsupported lifecycle status %q", ErrInvalidStream, metadata.LifecycleStatus)
	}
	return metadata, nil
}

func (status LifecycleStatus) Valid() bool {
	switch status {
	case LifecycleUnknown, LifecycleScheduled, LifecycleLive, LifecycleEnded, LifecycleUnavailable:
		return true
	default:
		return false
	}
}

func durationMilliseconds(duration *time.Duration) *int64 {
	if duration == nil {
		return nil
	}
	milliseconds := duration.Milliseconds()
	return &milliseconds
}

func textPointer(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func timePointer(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func durationPointer(value pgtype.Int8) *time.Duration {
	if !value.Valid {
		return nil
	}
	duration := time.Duration(value.Int64) * time.Millisecond
	return &duration
}

func isYouTubeVideoIDConflict(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) &&
		postgresError.Code == "23505" &&
		postgresError.ConstraintName == "streams_youtube_video_id_key"
}

func mapReadResult(operation string, stream Stream, err error) (Stream, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return Stream{}, ErrNotFound
	}
	if err != nil {
		return Stream{}, fmt.Errorf("%s: %w", operation, err)
	}
	return stream, nil
}
