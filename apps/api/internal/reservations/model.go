package reservations

import (
	"errors"
	"fmt"
)

var ErrInvalidTransition = errors.New("invalid reservation transition")

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

var transitions = map[State]map[Event]State{
	StateScheduled: {
		EventMonitor:                    StateMonitoring,
		EventBroadcastStarted:           StateLive,
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
