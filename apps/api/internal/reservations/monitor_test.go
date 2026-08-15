package reservations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/streams"
)

func TestMonitorEmitsSafeStructuredFailureMetric(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	repository := &monitorRepositoryStub{claimed: &ClaimedReservation{
		Reservation: Reservation{
			ID:             uuid.MustParse("10000000-0000-0000-0000-000000000001"),
			YouTubeVideoID: "fixture1234",
			SourceURL:      "https://www.youtube.com/watch?v=fixture1234",
			State:          StateMonitoring,
			MonitorAttempt: 2,
		},
	}}
	provider := metadataProviderStub{err: errors.New(
		"Authorization: Bearer auth-secret continuation=raw-continuation chat=private-body",
	)}
	var output bytes.Buffer
	monitor := NewMonitor(
		repository,
		provider,
		"monitor-worker",
		func() time.Time { return now },
		time.Minute,
		NewJSONMonitorObserver(&output),
	)

	didWork, err := monitor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !didWork {
		t.Fatal("RunOnce did not report work")
	}

	var metric map[string]any
	if err := json.Unmarshal(output.Bytes(), &metric); err != nil {
		t.Fatalf("decode metric: %v", err)
	}
	if metric["event"] != "reservation_monitor_check" ||
		metric["reservation_id"] != repository.claimed.ID.String() ||
		metric["attempt"] != float64(2) ||
		metric["outcome"] != "failed" ||
		metric["error_code"] != "YOUTUBE_TEMPORARILY_UNAVAILABLE" {
		t.Fatalf("unexpected metric: %#v", metric)
	}
	if duration, ok := metric["duration_seconds"].(float64); !ok || duration < 0 {
		t.Fatalf("invalid duration: %#v", metric["duration_seconds"])
	}
	for _, secret := range []string{"auth-secret", "raw-continuation", "private-body"} {
		if strings.Contains(output.String(), secret) {
			t.Fatalf("metric leaked %q: %s", secret, output.String())
		}
	}
	if repository.failureCode != "YOUTUBE_TEMPORARILY_UNAVAILABLE" ||
		repository.failureMessage != "YouTube metadata is temporarily unavailable." {
		t.Fatalf("unsafe durable failure: %q %q", repository.failureCode, repository.failureMessage)
	}
}

type monitorRepositoryStub struct {
	claimed        *ClaimedReservation
	failureCode    string
	failureMessage string
}

func (repository *monitorRepositoryStub) ClaimDue(
	context.Context,
	string,
	time.Time,
	time.Duration,
) (*ClaimedReservation, error) {
	return repository.claimed, nil
}

func (*monitorRepositoryStub) ApplyMetadata(
	context.Context,
	ClaimedReservation,
	streams.Metadata,
	time.Time,
) error {
	return nil
}

func (*monitorRepositoryStub) SyncCollection(context.Context, ClaimedReservation, time.Time) error {
	return nil
}

func (repository *monitorRepositoryStub) RecordFailure(
	_ context.Context,
	_ ClaimedReservation,
	_ time.Time,
	code string,
	message string,
	_ bool,
) error {
	repository.failureCode = code
	repository.failureMessage = message
	return nil
}

type metadataProviderStub struct{ err error }

func (provider metadataProviderStub) Fetch(context.Context, string) (streams.Metadata, error) {
	return streams.Metadata{}, provider.err
}
