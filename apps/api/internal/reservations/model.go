package reservations

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidTransition = errors.New("invalid reservation transition")
var ErrLeaseLost = errors.New("reservation lease is no longer owned by this worker")
var ErrNotFound = errors.New("reservation not found")

type State string

const (
	StateScheduled         State = "scheduled"
	StateMonitoring        State = "monitoring"
	StateLive              State = "live"
	StateWaitingForArchive State = "waiting_for_archive"
	StateCollecting        State = "collecting"
	StateCompleted         State = "completed"
	StateFailed            State = "failed"
	StateCanceled          State = "canceled"
)

type Event string

const (
	EventMonitor                    Event = "monitor"
	EventBroadcastStarted           Event = "broadcast_started"
	EventBroadcastEnded             Event = "broadcast_ended"
	EventArchiveReady               Event = "archive_ready"
	EventCollectionSucceeded        Event = "collection_succeeded"
	EventCancel                     Event = "cancel"
	EventTransientMonitoringFailure Event = "transient_monitoring_failure"
	EventPermanentMonitoringFailure Event = "permanent_monitoring_failure"
	EventCollectionFailed           Event = "collection_failed"
)

type Reservation struct {
	ID                 uuid.UUID
	YouTubeVideoID     string
	SourceURL          string
	State              State
	ScheduledStartAt   *time.Time
	ActualStartAt      *time.Time
	ActualEndAt        *time.Time
	NextCheckAt        time.Time
	LastCheckedAt      *time.Time
	MonitorAttempt     int
	LastErrorCode      *string
	LastErrorMessage   *string
	LastErrorRetryable *bool
	StreamID           *uuid.UUID
	CollectionJobID    *uuid.UUID
}

type Lease struct {
	WorkerID    string
	HeartbeatAt time.Time
	ExpiresAt   time.Time
	Revision    int64
}

type ClaimedReservation struct {
	Reservation
	Lease Lease
}

var transitions = map[State]map[Event]State{
	StateScheduled: {
		EventMonitor:                    StateMonitoring,
		EventBroadcastStarted:           StateLive,
		EventBroadcastEnded:             StateWaitingForArchive,
		EventCancel:                     StateCanceled,
		EventTransientMonitoringFailure: StateScheduled,
		EventPermanentMonitoringFailure: StateFailed,
	},
	StateMonitoring: {
		EventBroadcastStarted:           StateLive,
		EventBroadcastEnded:             StateWaitingForArchive,
		EventCancel:                     StateCanceled,
		EventTransientMonitoringFailure: StateMonitoring,
		EventPermanentMonitoringFailure: StateFailed,
	},
	StateLive: {
		EventBroadcastEnded:             StateWaitingForArchive,
		EventCancel:                     StateCanceled,
		EventTransientMonitoringFailure: StateLive,
		EventPermanentMonitoringFailure: StateFailed,
	},
	StateWaitingForArchive: {
		EventArchiveReady:               StateCollecting,
		EventCancel:                     StateCanceled,
		EventTransientMonitoringFailure: StateWaitingForArchive,
		EventPermanentMonitoringFailure: StateFailed,
	},
	StateCollecting: {
		EventCollectionSucceeded: StateCompleted,
		EventCollectionFailed:    StateCollecting,
	},
}

func Transition(from State, event Event) (State, error) {
	to, ok := transitions[from][event]
	if !ok {
		return from, fmt.Errorf("%w: %s cannot handle %s", ErrInvalidTransition, from, event)
	}
	return to, nil
}
