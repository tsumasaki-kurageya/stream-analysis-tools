//go:build integration

package reservations

import (
	"context"
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

func TestPostgresReservationSchemaEnforcesLifecycleOwnership(t *testing.T) {
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
	if err := database.ApplyMigrations(ctx, pool, os.DirFS(reservationMigrationDirectory(t))); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	streamID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO stream.streams (
			id, youtube_video_id, canonical_url, title, channel_id, channel_title,
			lifecycle_status, metadata_fetched_at
		) VALUES ($1, 'reserve0001', 'https://www.youtube.com/watch?v=reserve0001',
		          'Reserved stream', 'channel', 'Channel', 'ended', CURRENT_TIMESTAMP)
	`, streamID); err != nil {
		t.Fatalf("insert stream: %v", err)
	}

	firstReservationID := insertScheduledReservation(t, ctx, pool, "reserve0001")
	if _, err := pool.Exec(ctx, `
		INSERT INTO reservation.reservations (youtube_video_id, source_url, state, next_check_at)
		VALUES ('reserve0001', 'https://www.youtube.com/watch?v=reserve0001', 'monitoring', CURRENT_TIMESTAMP)
	`); err == nil {
		t.Fatal("expected one active reservation per video")
	}

	if _, err := pool.Exec(ctx, `
		UPDATE reservation.reservations
		SET state = 'canceled', canceled_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`, firstReservationID); err != nil {
		t.Fatalf("cancel first reservation: %v", err)
	}
	activeReservationID := insertScheduledReservation(t, ctx, pool, "reserve0001")

	if _, err := pool.Exec(ctx, `
		UPDATE reservation.reservations SET state = 'collecting' WHERE id = $1
	`, activeReservationID); err == nil {
		t.Fatal("expected collecting state to require stream and collection job")
	}

	var jobID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO collection.collection_jobs (stream_id, reservation_id, kind)
		VALUES ($1, $2, 'chat_replay')
		RETURNING id
	`, streamID, activeReservationID).Scan(&jobID); err != nil {
		t.Fatalf("insert reservation collection job: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE reservation.reservations
		SET state = 'collecting', stream_id = $2, collection_job_id = $3
		WHERE id = $1
	`, activeReservationID, streamID, jobID); err != nil {
		t.Fatalf("link reservation collection job: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO collection.collection_jobs (stream_id, reservation_id, kind)
		VALUES ($1, $2, 'chat_replay')
	`, streamID, activeReservationID); err == nil {
		t.Fatal("expected one collection job per reservation")
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO reservation.reservation_transitions (
			reservation_id, from_state, to_state, reason_code, facts
		) VALUES ($1, 'waiting_for_archive', 'collecting', 'archive_ready', '{"archive":"ready"}')
	`, activeReservationID); err != nil {
		t.Fatalf("record reservation transition: %v", err)
	}
	var transitionCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM reservation.reservation_transitions WHERE reservation_id = $1
	`, activeReservationID).Scan(&transitionCount); err != nil {
		t.Fatalf("count reservation transitions: %v", err)
	}
	if transitionCount != 1 {
		t.Fatalf("transition count = %d, want 1", transitionCount)
	}
}

func TestPostgresReservationRepositoryCreatesListsAndCancels(t *testing.T) {
	ctx, pool := newReservationTestPool(t)
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	repository := NewPostgresRepository(pool)

	created, err := repository.Create(ctx, Reservation{
		YouTubeVideoID: "publicapi01", SourceURL: "https://www.youtube.com/watch?v=publicapi01",
		State: StateScheduled, NextCheckAt: now,
	})
	if err != nil {
		t.Fatalf("create reservation: %v", err)
	}
	if !created.CreatedAt.Equal(now) || !created.CanCancel() {
		t.Fatalf("unexpected created reservation: %+v", created)
	}
	if _, err := repository.Create(ctx, Reservation{
		YouTubeVideoID: "publicapi01", SourceURL: created.SourceURL,
		State: StateScheduled, NextCheckAt: now,
	}); err != ErrAlreadyActive {
		t.Fatalf("duplicate error = %v, want ErrAlreadyActive", err)
	}

	items, total, err := repository.List(ctx, ListOptions{Limit: 20, Offset: 0})
	if err != nil || total != 1 || len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("unexpected list: items=%+v total=%d err=%v", items, total, err)
	}
	canceled, err := repository.Cancel(ctx, created.ID, now.Add(time.Minute))
	if err != nil || canceled.State != StateCanceled || canceled.CanCancel() {
		t.Fatalf("unexpected canceled reservation: %+v err=%v", canceled, err)
	}
	if _, err := repository.Cancel(ctx, created.ID, now.Add(2*time.Minute)); err != ErrNotCancellable {
		t.Fatalf("second cancel error = %v, want ErrNotCancellable", err)
	}
	var transitions int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM reservation.reservation_transitions WHERE reservation_id = $1
	`, created.ID).Scan(&transitions); err != nil || transitions != 2 {
		t.Fatalf("transition count = %d, err=%v", transitions, err)
	}
}

func insertScheduledReservation(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	videoID string,
) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO reservation.reservations (youtube_video_id, source_url, state, next_check_at)
		VALUES ($1, 'https://www.youtube.com/watch?v=' || $1, 'scheduled', CURRENT_TIMESTAMP)
		RETURNING id
	`, videoID).Scan(&id); err != nil {
		t.Fatalf("insert scheduled reservation: %v", err)
	}
	return id
}

func reservationMigrationDirectory(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../../migrations"))
}
