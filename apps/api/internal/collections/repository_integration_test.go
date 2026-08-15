//go:build integration

package collections

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
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
	}{
		{uuid.MustParse("00000000-0000-4000-8000-000000000002"), -250},
		{uuid.MustParse("00000000-0000-4000-8000-000000000003"), 1000},
		{uuid.MustParse("00000000-0000-4000-8000-000000000004"), 1000},
	}
	for index, message := range messages {
		if _, err := pool.Exec(ctx, `
			INSERT INTO chat.chat_messages (
				id, stream_id, collection_job_id, source, external_message_id,
				author_display_name, message_text, published_at, offset_milliseconds, message_type
			) VALUES ($1, $2, $3, 'youtube_chat_replay', $4, 'Viewer', $5,
			          CURRENT_TIMESTAMP, $6, 'text')
		`, message.id, streamID, retried.ID, "external-"+message.id.String(), "message", message.offset); err != nil {
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
}

func collectionMigrationDirectory(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../../migrations"))
}
