//go:build integration

package collections

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/database"
)

func TestPostgresCollectionInterfaces(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	container, err := postgres.Run(
		ctx,
		"postgres:18.4-bookworm",
		postgres.WithDatabase("stream_analysis_test"),
		postgres.WithUsername("stream_analysis"),
		postgres.WithPassword("stream_analysis_test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start PostgreSQL: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Errorf("terminate PostgreSQL: %v", err)
		}
	})

	connectionString, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("PostgreSQL connection string: %v", err)
	}
	pool, err := pgxpool.New(ctx, connectionString)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := database.ApplyMigrations(ctx, pool, os.DirFS(collectionMigrationDirectory(t))); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	streamID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO stream.streams (
			id, youtube_video_id, canonical_url, title, channel_id, channel_title,
			lifecycle_status, metadata_fetched_at
		) VALUES ($1, 'integration1', 'https://www.youtube.com/watch?v=integration1',
		          'Integration stream', 'channel', 'Channel', 'ended', CURRENT_TIMESTAMP)
	`, streamID); err != nil {
		t.Fatalf("insert stream: %v", err)
	}

	repository := NewPostgresRepository(pool)
	first, err := repository.Start(ctx, streamID)
	if err != nil {
		t.Fatalf("start collection: %v", err)
	}
	duplicate, err := repository.Start(ctx, streamID)
	if err != nil {
		t.Fatalf("repeat collection start: %v", err)
	}
	if duplicate.ID != first.ID {
		t.Fatalf("collection start was not idempotent: %s != %s", duplicate.ID, first.ID)
	}

	finishedAt := time.Date(2026, 8, 15, 2, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `
		UPDATE collection.collection_jobs
		SET status = 'failed', attempt = 1, started_at = $2, finished_at = $2,
		    error_code = 'YTDLP_TIMEOUT', error_message = '/tmp/private/cookies.txt --proxy secret',
		    updated_at = $2
		WHERE id = $1
	`, first.ID, finishedAt); err != nil {
		t.Fatalf("fail collection job: %v", err)
	}
	failed, err := repository.Latest(ctx, streamID)
	if err != nil {
		t.Fatalf("read failed collection: %v", err)
	}
	if failed.Error == nil || !failed.Error.Retryable || failed.Error.Code != "YTDLP_TIMEOUT" ||
		failed.Error.Message != "Chat replay collection timed out." {
		t.Fatalf("unsafe or unstable failed response: %+v", failed.Error)
	}

	retried, err := repository.Retry(ctx, first.ID)
	if err != nil {
		t.Fatalf("retry collection: %v", err)
	}
	if retried.ID == first.ID || retried.StreamID != streamID || retried.Status != StatusQueued {
		t.Fatalf("unexpected retried collection: %+v", retried)
	}
	if _, err := repository.Retry(ctx, first.ID); !errors.Is(err, ErrActiveJob) {
		t.Fatalf("expected active job conflict, got %v", err)
	}

	messages := []struct {
		id     uuid.UUID
		offset int64
		text   string
	}{
		{uuid.MustParse("00000000-0000-4000-8000-000000000002"), -250, "Opening message"},
		{uuid.MustParse("00000000-0000-4000-8000-000000000003"), 1000, "Music starts"},
		{uuid.MustParse("00000000-0000-4000-8000-000000000004"), 1000, "the music continues"},
	}
	for index, message := range messages {
		if _, err := pool.Exec(ctx, `
			INSERT INTO chat.chat_messages (
				id, stream_id, collection_job_id, source, external_message_id,
				author_display_name, message_text, published_at, offset_milliseconds, message_type
			) VALUES ($1, $2, $3, 'youtube_chat_replay', $4, 'Viewer', $5,
			          CURRENT_TIMESTAMP, $6, 'text')
		`, message.id, streamID, retried.ID, "external-"+message.id.String(), message.text, message.offset); err != nil {
			t.Fatalf("insert chat message %d: %v", index, err)
		}
	}

	service := NewService(repository)
	firstPage, err := service.ListMessages(ctx, streamID, 2, "")
	if err != nil {
		t.Fatalf("list first chat page: %v", err)
	}
	if len(firstPage.Items) != 2 || firstPage.NextCursor == nil ||
		firstPage.Items[0].ID != messages[0].id || firstPage.Items[1].ID != messages[1].id {
		t.Fatalf("unexpected first chat page: %+v", firstPage)
	}
	secondPage, err := service.ListMessages(ctx, streamID, 2, *firstPage.NextCursor)
	if err != nil {
		t.Fatalf("list second chat page: %v", err)
	}
	if len(secondPage.Items) != 1 || secondPage.NextCursor != nil || secondPage.Items[0].ID != messages[2].id {
		t.Fatalf("unexpected second chat page: %+v", secondPage)
	}

	otherStreamID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO stream.streams (
			id, youtube_video_id, canonical_url, title, channel_id, channel_title,
			lifecycle_status, metadata_fetched_at
		) VALUES ($1, 'integration2', 'https://www.youtube.com/watch?v=integration2',
		          'Other integration stream', 'channel', 'Channel', 'ended', CURRENT_TIMESTAMP)
	`, otherStreamID); err != nil {
		t.Fatalf("insert other stream: %v", err)
	}
	otherJob, err := repository.Start(ctx, otherStreamID)
	if err != nil {
		t.Fatalf("start other collection: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO chat.chat_messages (
			stream_id, collection_job_id, source, external_message_id,
			author_display_name, message_text, published_at, offset_milliseconds, message_type
		) VALUES ($1, $2, 'youtube_chat_replay', 'other-music', 'Other viewer',
		          'Music from another stream', CURRENT_TIMESTAMP, 500, 'text')
	`, otherStreamID, otherJob.ID); err != nil {
		t.Fatalf("insert other stream chat: %v", err)
	}

	firstSearchPage, err := service.SearchMessages(ctx, streamID, "MUSIC", 1, "")
	if err != nil {
		t.Fatalf("search first chat page: %v", err)
	}
	if len(firstSearchPage.Items) != 1 || firstSearchPage.NextCursor == nil ||
		firstSearchPage.Items[0].ID != messages[1].id || firstSearchPage.Items[0].OffsetMilliseconds != 1000 {
		t.Fatalf("unexpected first search page: %+v", firstSearchPage)
	}
	secondSearchPage, err := service.SearchMessages(
		ctx,
		streamID,
		"MUSIC",
		1,
		*firstSearchPage.NextCursor,
	)
	if err != nil {
		t.Fatalf("search second chat page: %v", err)
	}
	if len(secondSearchPage.Items) != 1 || secondSearchPage.NextCursor != nil ||
		secondSearchPage.Items[0].ID != messages[2].id {
		t.Fatalf("unexpected second search page: %+v", secondSearchPage)
	}

	literalPercentID := uuid.MustParse("00000000-0000-4000-8000-000000000005")
	if _, err := pool.Exec(ctx, `
		INSERT INTO chat.chat_messages (
			id, stream_id, collection_job_id, source, external_message_id,
			author_display_name, message_text, published_at, offset_milliseconds, message_type
		) VALUES
			($1, $3, $4, 'youtube_chat_replay', 'literal-percent', 'Viewer',
			 'Score 100% complete', CURRENT_TIMESTAMP, 2000, 'text'),
			($2, $3, $4, 'youtube_chat_replay', 'wildcard-lookalike', 'Viewer',
			 'Score 1000 complete', CURRENT_TIMESTAMP, 3000, 'text')
	`,
		literalPercentID,
		uuid.MustParse("00000000-0000-4000-8000-000000000006"),
		streamID,
		retried.ID,
	); err != nil {
		t.Fatalf("insert literal search messages: %v", err)
	}
	literalSearch, err := service.SearchMessages(ctx, streamID, "100%", 10, "")
	if err != nil {
		t.Fatalf("search literal percent: %v", err)
	}
	if len(literalSearch.Items) != 1 || literalSearch.Items[0].ID != literalPercentID {
		t.Fatalf("search treated literal percent as a wildcard: %+v", literalSearch)
	}

	var cursorIndexExists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE schemaname = 'chat' AND indexname = 'chat_messages_stream_offset_id_idx'
		)
	`).Scan(&cursorIndexExists); err != nil {
		t.Fatalf("inspect cursor index: %v", err)
	}
	if !cursorIndexExists {
		t.Fatal("chat cursor index is missing")
	}

	var searchIndexExists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE schemaname = 'chat' AND indexname = 'chat_messages_message_text_trgm_idx'
		)
	`).Scan(&searchIndexExists); err != nil {
		t.Fatalf("inspect chat search index: %v", err)
	}
	if !searchIndexExists {
		t.Fatal("chat search index is missing")
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO chat.chat_messages (
			stream_id, collection_job_id, source, external_message_id,
			author_display_name, message_text, published_at, offset_milliseconds, message_type
		)
		SELECT $1, $2, 'youtube_chat_replay', 'plan-' || value, 'Plan viewer',
		       CASE WHEN value = 25000 THEN 'selective search needle' ELSE md5(value::text) END,
		       CURRENT_TIMESTAMP, 4000 + value, 'text'
		FROM generate_series(1, 50000) AS value
	`, streamID, retried.ID); err != nil {
		t.Fatalf("seed chat search plan data: %v", err)
	}
	if _, err := pool.Exec(ctx, "ANALYZE chat.chat_messages"); err != nil {
		t.Fatalf("analyze chat search data: %v", err)
	}
	planRows, err := pool.Query(ctx, `
		EXPLAIN (COSTS OFF)
		SELECT id, offset_milliseconds
		FROM chat.chat_messages
		WHERE stream_id = $1
		  AND message_text ILIKE '%search needle%' ESCAPE '\'
		  AND (offset_milliseconds, id) > (-9223372036854775808, '00000000-0000-0000-0000-000000000000')
		ORDER BY offset_milliseconds, id
		LIMIT 51
	`, streamID)
	if err != nil {
		t.Fatalf("explain chat search: %v", err)
	}
	defer planRows.Close()
	var plan strings.Builder
	for planRows.Next() {
		var line string
		if err := planRows.Scan(&line); err != nil {
			t.Fatalf("scan chat search plan: %v", err)
		}
		plan.WriteString(line)
		plan.WriteByte('\n')
	}
	if err := planRows.Err(); err != nil {
		t.Fatalf("iterate chat search plan: %v", err)
	}
	if !strings.Contains(plan.String(), "chat_messages_message_text_trgm_idx") {
		t.Fatalf("chat search plan did not use trigram index:\n%s", plan.String())
	}
}

func collectionMigrationDirectory(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../../migrations"))
}
