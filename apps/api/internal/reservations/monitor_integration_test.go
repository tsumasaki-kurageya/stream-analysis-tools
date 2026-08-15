//go:build integration

package reservations

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/database"
	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/streams"
)

func TestPostgresReservationClaimRecoversExpiredWorkWithoutTwoOwners(t *testing.T) {
	ctx, pool := newReservationTestPool(t)
	claimedAt := time.Date(2026, 8, 15, 6, 0, 0, 0, time.UTC)
	var reservationID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO reservation.reservations (
			youtube_video_id, source_url, state, next_check_at
		) VALUES ('monitor0001', 'https://www.youtube.com/watch?v=monitor0001', 'scheduled', $1)
		RETURNING id
	`, claimedAt).Scan(&reservationID); err != nil {
		t.Fatalf("insert due reservation: %v", err)
	}

	repository := NewPostgresRepository(pool)
	first, err := repository.ClaimDue(ctx, "monitor-a", claimedAt, time.Minute)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if first == nil || first.ID.String() != reservationID || first.Lease.WorkerID != "monitor-a" {
		t.Fatalf("unexpected first claim: %+v", first)
	}
	heartbeatAt := claimedAt.Add(30 * time.Second)
	active, err := repository.Heartbeat(ctx, *first, heartbeatAt, time.Minute)
	if err != nil {
		t.Fatalf("heartbeat active claim: %v", err)
	}
	if active.Lease.Revision <= first.Lease.Revision || !active.Lease.ExpiresAt.Equal(heartbeatAt.Add(time.Minute)) {
		t.Fatalf("heartbeat did not extend active claim: %+v", active)
	}

	concurrent, err := repository.ClaimDue(ctx, "monitor-b", heartbeatAt, time.Minute)
	if err != nil {
		t.Fatalf("concurrent claim: %v", err)
	}
	if concurrent != nil {
		t.Fatalf("reservation was claimed by two workers: %+v", concurrent)
	}

	recoveredAt := claimedAt.Add(2 * time.Minute)
	recovered, err := repository.ClaimDue(ctx, "monitor-b", recoveredAt, time.Minute)
	if err != nil {
		t.Fatalf("recover expired claim: %v", err)
	}
	if recovered == nil || recovered.ID != first.ID || recovered.Lease.Revision <= first.Lease.Revision {
		t.Fatalf("unexpected recovered claim: %+v", recovered)
	}
	if _, err := repository.Heartbeat(ctx, *first, recoveredAt, time.Minute); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("stale heartbeat error = %v, want ErrLeaseLost", err)
	}
}

func TestMonitorWaitsForArchiveThenCreatesExactlyOneCollectionJob(t *testing.T) {
	ctx, pool := newReservationTestPool(t)
	currentTime := time.Date(2026, 8, 15, 7, 0, 0, 0, time.UTC)
	var reservationID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO reservation.reservations (
			youtube_video_id, source_url, state, next_check_at
		) VALUES ('archive0001', 'https://www.youtube.com/watch?v=archive0001', 'monitoring', $1)
		RETURNING id
	`, currentTime).Scan(&reservationID); err != nil {
		t.Fatalf("insert monitored reservation: %v", err)
	}

	actualStart := currentTime.Add(-2 * time.Hour)
	actualEnd := currentTime.Add(-time.Hour)
	duration := time.Hour
	provider := &reservationSequenceProvider{results: []streams.Metadata{
		{
			Title: "Ended stream", ChannelID: "UC-monitor", ChannelTitle: "Monitor channel",
			ActualStartAt: &actualStart, ActualEndAt: &actualEnd,
			LifecycleStatus: streams.LifecycleEnded, MetadataFetchedAt: currentTime,
		},
		{
			Title: "Ended stream", ChannelID: "UC-monitor", ChannelTitle: "Monitor channel",
			ActualStartAt: &actualStart, ActualEndAt: &actualEnd, Duration: &duration,
			LifecycleStatus: streams.LifecycleEnded, MetadataFetchedAt: currentTime.Add(2 * time.Minute),
		},
	}}
	repository := NewPostgresRepository(pool)
	monitor := NewMonitor(repository, provider, "monitor-worker", func() time.Time { return currentTime }, time.Minute)

	didWork, err := monitor.RunOnce(ctx)
	if err != nil {
		t.Fatalf("monitor ended stream before archive readiness: %v", err)
	}
	if !didWork {
		t.Fatal("monitor did not claim due reservation")
	}
	waiting, err := repository.Get(ctx, reservationID)
	if err != nil {
		t.Fatalf("get waiting reservation: %v", err)
	}
	if waiting.State != StateWaitingForArchive || waiting.CollectionJobID != nil || waiting.StreamID == nil {
		t.Fatalf("unexpected waiting reservation: %+v", waiting)
	}
	var existingJobID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO collection.collection_jobs (stream_id, kind)
		VALUES ($1, 'chat_replay')
		RETURNING id
	`, *waiting.StreamID).Scan(&existingJobID); err != nil {
		t.Fatalf("insert existing M2 collection job: %v", err)
	}

	currentTime = waiting.NextCheckAt
	if _, err := monitor.RunOnce(ctx); err != nil {
		t.Fatalf("monitor archive readiness: %v", err)
	}
	collecting, err := repository.Get(ctx, reservationID)
	if err != nil {
		t.Fatalf("get collecting reservation: %v", err)
	}
	if collecting.State != StateCollecting || collecting.CollectionJobID == nil || collecting.StreamID == nil {
		t.Fatalf("unexpected collecting reservation: %+v", collecting)
	}
	if *collecting.CollectionJobID != existingJobID {
		t.Fatalf("monitor duplicated existing collection job: got %s, want %s", *collecting.CollectionJobID, existingJobID)
	}

	currentTime = collecting.NextCheckAt
	if _, err := monitor.RunOnce(ctx); err != nil {
		t.Fatalf("recheck collecting reservation: %v", err)
	}
	var jobCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM collection.collection_jobs WHERE reservation_id = $1
	`, reservationID).Scan(&jobCount); err != nil {
		t.Fatalf("count automatic collection jobs: %v", err)
	}
	if jobCount != 1 {
		t.Fatalf("automatic collection job count = %d, want 1", jobCount)
	}

	if _, err := pool.Exec(ctx, `
		UPDATE collection.collection_jobs
		SET status = 'succeeded', started_at = $2, finished_at = $2, updated_at = $2
		WHERE id = $1
	`, *collecting.CollectionJobID, currentTime); err != nil {
		t.Fatalf("complete automatic collection job: %v", err)
	}
	currentTime = currentTime.Add(time.Minute)
	if _, err := monitor.RunOnce(ctx); err != nil {
		t.Fatalf("sync successful collection: %v", err)
	}
	completed, err := repository.Get(ctx, reservationID)
	if err != nil {
		t.Fatalf("get completed reservation: %v", err)
	}
	if completed.State != StateCompleted {
		t.Fatalf("successful collection did not complete reservation: %+v", completed)
	}
}

func TestMonitorBoundsTransientProviderRetries(t *testing.T) {
	ctx, pool := newReservationTestPool(t)
	currentTime := time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC)
	var reservationID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO reservation.reservations (
			youtube_video_id, source_url, state, next_check_at
		) VALUES ('retry000001', 'https://www.youtube.com/watch?v=retry000001', 'monitoring', $1)
		RETURNING id
	`, currentTime).Scan(&reservationID); err != nil {
		t.Fatalf("insert retrying reservation: %v", err)
	}

	repository := NewPostgresRepository(pool)
	provider := &reservationSequenceProvider{err: streams.ErrMetadataProviderUnavailable}
	monitor := NewMonitor(repository, provider, "retry-worker", func() time.Time { return currentTime }, time.Minute)

	for attempt := 1; attempt <= MaxMonitorAttempts; attempt++ {
		didWork, err := monitor.RunOnce(ctx)
		if err != nil {
			t.Fatalf("monitor retry %d: %v", attempt, err)
		}
		if !didWork {
			t.Fatalf("monitor retry %d did not claim work", attempt)
		}
		reservation, err := repository.Get(ctx, reservationID)
		if err != nil {
			t.Fatalf("get reservation after retry %d: %v", attempt, err)
		}
		if reservation.MonitorAttempt != attempt {
			t.Fatalf("monitor attempt = %d, want %d", reservation.MonitorAttempt, attempt)
		}
		if attempt < MaxMonitorAttempts {
			if reservation.State != StateMonitoring || reservation.LastErrorRetryable == nil || !*reservation.LastErrorRetryable {
				t.Fatalf("unexpected transient retry state: %+v", reservation)
			}
			if reservation.NextCheckAt.Sub(currentTime) > 30*time.Minute {
				t.Fatalf("retry delay exceeded bound: %s", reservation.NextCheckAt.Sub(currentTime))
			}
			currentTime = reservation.NextCheckAt
			continue
		}
		if reservation.State != StateFailed || reservation.LastErrorRetryable == nil || *reservation.LastErrorRetryable {
			t.Fatalf("retry exhaustion did not fail reservation: %+v", reservation)
		}
	}
}

func TestMonitorDetectsScheduledLiveAndEndedBroadcastStates(t *testing.T) {
	ctx, pool := newReservationTestPool(t)
	currentTime := time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)
	scheduledStart := currentTime.Add(10 * time.Minute)
	actualStart := currentTime.Add(time.Minute)
	actualEnd := currentTime.Add(3 * time.Minute)
	var reservationID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO reservation.reservations (
			youtube_video_id, source_url, state, scheduled_start_at, next_check_at
		) VALUES ('states00001', 'https://www.youtube.com/watch?v=states00001', 'scheduled', $1, $2)
		RETURNING id
	`, scheduledStart, currentTime).Scan(&reservationID); err != nil {
		t.Fatalf("insert scheduled reservation: %v", err)
	}

	metadata := func(status streams.LifecycleStatus, fetchedAt time.Time) streams.Metadata {
		return streams.Metadata{
			Title: "Observed stream", ChannelID: "UC-states", ChannelTitle: "States channel",
			ScheduledStartAt: &scheduledStart, LifecycleStatus: status, MetadataFetchedAt: fetchedAt,
		}
	}
	scheduled := metadata(streams.LifecycleScheduled, currentTime)
	live := metadata(streams.LifecycleLive, currentTime.Add(time.Minute))
	live.ActualStartAt = &actualStart
	ended := metadata(streams.LifecycleEnded, currentTime.Add(3*time.Minute))
	ended.ActualStartAt = &actualStart
	ended.ActualEndAt = &actualEnd
	provider := &reservationSequenceProvider{results: []streams.Metadata{scheduled, live, ended}}
	repository := NewPostgresRepository(pool)
	monitor := NewMonitor(repository, provider, "state-worker", func() time.Time { return currentTime }, time.Minute)

	if _, err := monitor.RunOnce(ctx); err != nil {
		t.Fatalf("observe scheduled broadcast: %v", err)
	}
	observed, err := repository.Get(ctx, reservationID)
	if err != nil {
		t.Fatalf("get scheduled observation: %v", err)
	}
	if observed.State != StateScheduled || !observed.NextCheckAt.Equal(currentTime.Add(time.Minute)) {
		t.Fatalf("unexpected scheduled observation: %+v", observed)
	}

	currentTime = observed.NextCheckAt
	if _, err := monitor.RunOnce(ctx); err != nil {
		t.Fatalf("observe live broadcast: %v", err)
	}
	observed, err = repository.Get(ctx, reservationID)
	if err != nil {
		t.Fatalf("get live observation: %v", err)
	}
	if observed.State != StateLive || !observed.NextCheckAt.Equal(currentTime.Add(2*time.Minute)) {
		t.Fatalf("unexpected live observation: %+v", observed)
	}

	currentTime = observed.NextCheckAt
	if _, err := monitor.RunOnce(ctx); err != nil {
		t.Fatalf("observe ended broadcast: %v", err)
	}
	observed, err = repository.Get(ctx, reservationID)
	if err != nil {
		t.Fatalf("get ended observation: %v", err)
	}
	if observed.State != StateWaitingForArchive || observed.StreamID == nil {
		t.Fatalf("unexpected ended observation: %+v", observed)
	}
}

type reservationSequenceProvider struct {
	results []streams.Metadata
	err     error
	calls   int
}

func (provider *reservationSequenceProvider) Fetch(context.Context, string) (streams.Metadata, error) {
	if provider.err != nil {
		return streams.Metadata{}, provider.err
	}
	index := provider.calls
	provider.calls++
	if index >= len(provider.results) {
		index = len(provider.results) - 1
	}
	return provider.results[index], nil
}

func newReservationTestPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	t.Cleanup(cancel)

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
	return ctx, pool
}
