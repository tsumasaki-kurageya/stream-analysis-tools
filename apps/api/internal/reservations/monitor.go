package reservations

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/streams"
)

const MaxMonitorAttempts = 5

type MonitorRepository interface {
	ClaimDue(context.Context, string, time.Time, time.Duration) (*ClaimedReservation, error)
	ApplyMetadata(context.Context, ClaimedReservation, streams.Metadata, time.Time) error
	SyncCollection(context.Context, ClaimedReservation, time.Time) error
	RecordFailure(context.Context, ClaimedReservation, time.Time, string, string, bool) error
}

type Monitor struct {
	repository    MonitorRepository
	provider      streams.MetadataProvider
	workerID      string
	clock         func() time.Time
	leaseDuration time.Duration
	observer      MonitorObserver
}

type MonitorMetric struct {
	Event           string  `json:"event"`
	ReservationID   string  `json:"reservation_id"`
	State           State   `json:"state"`
	Attempt         int     `json:"attempt"`
	Outcome         string  `json:"outcome"`
	DurationSeconds float64 `json:"duration_seconds"`
	ErrorCode       string  `json:"error_code,omitempty"`
}

type MonitorObserver interface {
	Observe(MonitorMetric)
}

type discardMonitorObserver struct{}

func (discardMonitorObserver) Observe(MonitorMetric) {}

type JSONMonitorObserver struct {
	encoder *json.Encoder
	mu      sync.Mutex
}

func NewJSONMonitorObserver(output io.Writer) *JSONMonitorObserver {
	return &JSONMonitorObserver{encoder: json.NewEncoder(output)}
}

func (observer *JSONMonitorObserver) Observe(metric MonitorMetric) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	_ = observer.encoder.Encode(metric)
}

func NewMonitor(
	repository MonitorRepository,
	provider streams.MetadataProvider,
	workerID string,
	clock func() time.Time,
	leaseDuration time.Duration,
	observers ...MonitorObserver,
) *Monitor {
	observer := MonitorObserver(discardMonitorObserver{})
	if len(observers) > 0 && observers[0] != nil {
		observer = observers[0]
	}
	return &Monitor{
		repository: repository, provider: provider, workerID: workerID,
		clock: clock, leaseDuration: leaseDuration, observer: observer,
	}
}

func (monitor *Monitor) RunOnce(ctx context.Context) (bool, error) {
	startedAt := time.Now()
	checkedAt := monitor.clock().UTC()
	claimed, err := monitor.repository.ClaimDue(
		ctx,
		monitor.workerID,
		checkedAt,
		monitor.leaseDuration,
	)
	if err != nil || claimed == nil {
		return false, err
	}
	outcome := "checked"
	errorCode := ""
	defer func() {
		monitor.observer.Observe(MonitorMetric{
			Event: "reservation_monitor_check", ReservationID: claimed.ID.String(),
			State: claimed.State, Attempt: claimed.MonitorAttempt, Outcome: outcome,
			DurationSeconds: time.Since(startedAt).Seconds(), ErrorCode: errorCode,
		})
	}()
	if claimed.State == StateCollecting {
		err := monitor.repository.SyncCollection(ctx, *claimed, checkedAt)
		if err != nil {
			outcome, errorCode = "failed", "RESERVATION_PERSIST_FAILED"
		} else {
			outcome = "collection_synced"
		}
		return true, err
	}
	if monitor.provider == nil {
		outcome, errorCode = "failed", "RESERVATION_MONITOR_MISCONFIGURED"
		return true, errors.New("reservation metadata provider is required")
	}
	metadata, err := monitor.provider.Fetch(ctx, claimed.YouTubeVideoID)
	if err != nil {
		code := "YOUTUBE_TEMPORARILY_UNAVAILABLE"
		message := "YouTube metadata is temporarily unavailable."
		retryable := true
		if errors.Is(err, streams.ErrVideoUnavailable) {
			code = "RESERVATION_VIDEO_NOT_FOUND"
			message = "The YouTube video is unavailable."
			retryable = false
		}
		outcome, errorCode = "failed", code
		recordError := monitor.repository.RecordFailure(
			ctx,
			*claimed,
			checkedAt,
			code,
			message,
			retryable,
		)
		if recordError != nil {
			errorCode = "RESERVATION_PERSIST_FAILED"
		}
		return true, recordError
	}
	metadata.YouTubeVideoID = claimed.YouTubeVideoID
	metadata.CanonicalURL = claimed.SourceURL
	err = monitor.repository.ApplyMetadata(ctx, *claimed, metadata, checkedAt)
	if err != nil {
		outcome, errorCode = "failed", "RESERVATION_PERSIST_FAILED"
	} else {
		outcome = "metadata_applied"
	}
	return true, err
}
