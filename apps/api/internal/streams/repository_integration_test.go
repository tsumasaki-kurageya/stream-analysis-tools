//go:build integration

package streams

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

func TestPostgresRepository(t *testing.T) {
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
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping PostgreSQL: %v", err)
	}

	migrationFS := os.DirFS(migrationDirectory(t))
	if err := database.ApplyMigrations(ctx, pool, migrationFS); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if err := database.ApplyMigrations(ctx, pool, migrationFS); err != nil {
		t.Fatalf("reapply migrations: %v", err)
	}

	repository := NewPostgresRepository(pool)
	firstMetadata := streamMetadataFixture("video00001A", time.Date(2026, 8, 11, 2, 0, 0, 0, time.UTC))
	first, err := repository.Create(ctx, firstMetadata)
	if err != nil {
		t.Fatalf("create first stream: %v", err)
	}
	if first.ID == uuid.Nil {
		t.Fatal("created stream has a nil ID")
	}
	if first.YouTubeVideoID != firstMetadata.YouTubeVideoID || first.LifecycleStatus != LifecycleEnded {
		t.Fatalf("unexpected created stream: %+v", first)
	}

	_, err = repository.Create(ctx, firstMetadata)
	if !errors.Is(err, ErrYouTubeVideoIDExists) {
		t.Fatalf("expected duplicate video ID error, got %v", err)
	}

	updatedMetadata := firstMetadata
	updatedMetadata.Title = "Updated stream title"
	updatedMetadata.MetadataFetchedAt = firstMetadata.MetadataFetchedAt.Add(time.Minute)
	updated, err := repository.Upsert(ctx, updatedMetadata)
	if err != nil {
		t.Fatalf("upsert first stream: %v", err)
	}
	if updated.ID != first.ID {
		t.Fatalf("upsert changed internal ID: %s != %s", updated.ID, first.ID)
	}
	if updated.Title != updatedMetadata.Title {
		t.Fatalf("upsert did not refresh metadata: %+v", updated)
	}

	byID, err := repository.Get(ctx, first.ID)
	if err != nil {
		t.Fatalf("get stream by ID: %v", err)
	}
	byVideoID, err := repository.GetByYouTubeVideoID(ctx, firstMetadata.YouTubeVideoID)
	if err != nil {
		t.Fatalf("get stream by video ID: %v", err)
	}
	if byID.ID != updated.ID || byVideoID.ID != updated.ID {
		t.Fatalf("detail queries returned different streams: by ID=%s by video ID=%s", byID.ID, byVideoID.ID)
	}

	secondMetadata := streamMetadataFixture("video00002B", firstMetadata.MetadataFetchedAt.Add(2*time.Minute))
	secondMetadata.Title = "Second stream"
	second, err := repository.Create(ctx, secondMetadata)
	if err != nil {
		t.Fatalf("create second stream: %v", err)
	}

	listed, err := repository.List(ctx, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("list streams: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected two streams, got %d", len(listed))
	}
	if listed[0].CreatedAt.Before(listed[1].CreatedAt) {
		t.Fatalf("streams are not ordered newest first: %s, %s", listed[0].CreatedAt, listed[1].CreatedAt)
	}
	listedIDs := map[uuid.UUID]bool{listed[0].ID: true, listed[1].ID: true}
	if !listedIDs[first.ID] || !listedIDs[second.ID] {
		t.Fatalf("list did not return both streams: %s, %s", listed[0].ID, listed[1].ID)
	}

	offsetPage, err := repository.List(ctx, ListOptions{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("list streams with offset: %v", err)
	}
	if len(offsetPage) != 1 || offsetPage[0].ID != listed[1].ID {
		t.Fatalf("unexpected offset page: %+v", offsetPage)
	}

	_, err = repository.Get(ctx, uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found error, got %v", err)
	}

	var streamCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stream.streams").Scan(&streamCount); err != nil {
		t.Fatalf("count streams: %v", err)
	}
	if streamCount != 2 {
		t.Fatalf("expected two unique rows, got %d", streamCount)
	}

	var expectedIndexCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_indexes
		WHERE schemaname = 'stream'
		  AND indexname IN ('streams_youtube_video_id_key', 'streams_created_at_id_idx')
	`).Scan(&expectedIndexCount); err != nil {
		t.Fatalf("inspect stream indexes: %v", err)
	}
	if expectedIndexCount != 2 {
		t.Fatalf("expected video ID and list indexes, got %d", expectedIndexCount)
	}

	downSQL, err := os.ReadFile(filepath.Join(migrationDirectory(t), "000001_create_streams.down.sql"))
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(downSQL)); err != nil {
		t.Fatalf("apply down migration: %v", err)
	}
	var tableRemoved bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('stream.streams') IS NULL").Scan(&tableRemoved); err != nil {
		t.Fatalf("check down migration: %v", err)
	}
	if !tableRemoved {
		t.Fatal("stream table remains after down migration")
	}
}

func streamMetadataFixture(videoID string, fetchedAt time.Time) Metadata {
	thumbnailURL := "https://i.ytimg.com/vi/" + videoID + "/hqdefault.jpg"
	actualStart := fetchedAt.Add(-2 * time.Hour)
	actualEnd := fetchedAt.Add(-time.Minute)
	duration := actualEnd.Sub(actualStart)
	return Metadata{
		YouTubeVideoID:    videoID,
		CanonicalURL:      "https://www.youtube.com/watch?v=" + videoID,
		Title:             "Fixture stream",
		ChannelID:         "channel-fixture",
		ChannelTitle:      "Fixture channel",
		ThumbnailURL:      &thumbnailURL,
		ActualStartAt:     &actualStart,
		ActualEndAt:       &actualEnd,
		Duration:          &duration,
		LifecycleStatus:   LifecycleEnded,
		MetadataFetchedAt: fetchedAt,
	}
}

func migrationDirectory(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../../migrations"))
}
