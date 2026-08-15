package reservations

import (
	"context"
	"errors"
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
}

func NewMonitor(
	repository MonitorRepository,
	provider streams.MetadataProvider,
	workerID string,
	clock func() time.Time,
	leaseDuration time.Duration,
) *Monitor {
	return &Monitor{
		repository: repository, provider: provider, workerID: workerID,
		clock: clock, leaseDuration: leaseDuration,
	}
}

func (monitor *Monitor) RunOnce(ctx context.Context) (bool, error) {
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
	if claimed.State == StateCollecting {
		return true, monitor.repository.SyncCollection(ctx, *claimed, checkedAt)
	}
	if monitor.provider == nil {
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
		return true, monitor.repository.RecordFailure(
			ctx,
			*claimed,
			checkedAt,
			code,
			message,
			retryable,
		)
	}
	metadata.YouTubeVideoID = claimed.YouTubeVideoID
	metadata.CanonicalURL = claimed.SourceURL
	return true, monitor.repository.ApplyMetadata(ctx, *claimed, metadata, checkedAt)
}
