package collections

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (repository *PostgresRepository) ChatActivity(
	ctx context.Context,
	streamID uuid.UUID,
	bucketSeconds int,
) ([]ActivityBucket, error) {
	var streamExists bool
	if err := repository.pool.QueryRow(
		ctx,
		"SELECT EXISTS (SELECT 1 FROM stream.streams WHERE id = $1)",
		streamID,
	).Scan(&streamExists); err != nil {
		return nil, fmt.Errorf("check stream for chat activity: %w", err)
	}
	if !streamExists {
		return nil, ErrStreamNotFound
	}

	bucketMilliseconds := int64(bucketSeconds) * 1000
	rows, err := repository.pool.Query(ctx, `
		WITH bounds AS (
			SELECT COALESCE(
				duration_ms,
				(SELECT MAX(offset_milliseconds) + 1 FROM chat.chat_messages WHERE stream_id = $1),
				0
			) AS duration_ms
			FROM stream.streams
			WHERE id = $1
		), counts AS (
			SELECT (GREATEST(offset_milliseconds, 0) / $2) * $2 AS bucket_start,
			       COUNT(*)::bigint AS message_count
			FROM chat.chat_messages
			WHERE stream_id = $1
			GROUP BY bucket_start
		)
		SELECT series.bucket_start,
		       COALESCE(counts.message_count, 0)::bigint
		FROM bounds
		CROSS JOIN LATERAL generate_series(
			0::bigint,
			GREATEST(bounds.duration_ms - 1, 0)::bigint,
			$2::bigint
		) AS series(bucket_start)
		LEFT JOIN counts ON counts.bucket_start = series.bucket_start
		WHERE bounds.duration_ms > 0
		ORDER BY series.bucket_start
	`, streamID, bucketMilliseconds)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrStreamNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query chat activity: %w", err)
	}
	defer rows.Close()

	items := make([]ActivityBucket, 0)
	for rows.Next() {
		var item ActivityBucket
		if err := rows.Scan(&item.StartOffsetMilliseconds, &item.MessageCount); err != nil {
			return nil, fmt.Errorf("scan chat activity bucket: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat activity buckets: %w", err)
	}
	return items, nil
}
