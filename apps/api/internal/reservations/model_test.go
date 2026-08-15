package reservations

import (
	"errors"
	"testing"
)

func TestTransitionAdvancesReservationThroughCollection(t *testing.T) {
	tests := []struct {
		name  string
		from  State
		event Event
		want  State
	}{
		{name: "approach scheduled start", from: StateScheduled, event: EventMonitor, want: StateMonitoring},
		{name: "detect live broadcast from schedule", from: StateScheduled, event: EventBroadcastStarted, want: StateLive},
		{name: "detect live broadcast while monitoring", from: StateMonitoring, event: EventBroadcastStarted, want: StateLive},
		{name: "detect ended broadcast while monitoring", from: StateMonitoring, event: EventBroadcastEnded, want: StateWaitingForArchive},
		{name: "detect ended live broadcast", from: StateLive, event: EventBroadcastEnded, want: StateWaitingForArchive},
		{name: "detect archive readiness", from: StateWaitingForArchive, event: EventArchiveReady, want: StateCollecting},
		{name: "observe successful collection", from: StateCollecting, event: EventCollectionSucceeded, want: StateCompleted},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Transition(test.from, test.event)
			if err != nil {
				t.Fatalf("transition returned an error: %v", err)
			}
			if got != test.want {
				t.Fatalf("transition = %q, want %q", got, test.want)
			}
		})
	}
}

func TestTransitionAllowsCancellationOnlyBeforeCollection(t *testing.T) {
	for _, state := range []State{
		StateScheduled,
		StateMonitoring,
		StateLive,
		StateWaitingForArchive,
	} {
		got, err := Transition(state, EventCancel)
		if err != nil {
			t.Fatalf("cancel %q returned an error: %v", state, err)
		}
		if got != StateCanceled {
			t.Fatalf("cancel %q = %q, want %q", state, got, StateCanceled)
		}
	}

	for _, state := range []State{StateCollecting, StateCompleted, StateFailed, StateCanceled} {
		got, err := Transition(state, EventCancel)
		if !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("cancel %q error = %v, want ErrInvalidTransition", state, err)
		}
		if got != state {
			t.Fatalf("rejected cancel %q returned state %q", state, got)
		}
	}
}

func TestTransitionSeparatesMonitoringAndCollectionFailures(t *testing.T) {
	monitoringStates := []State{
		StateScheduled,
		StateMonitoring,
		StateLive,
		StateWaitingForArchive,
	}
	for _, state := range monitoringStates {
		got, err := Transition(state, EventTransientMonitoringFailure)
		if err != nil {
			t.Fatalf("transient monitoring failure in %q returned an error: %v", state, err)
		}
		if got != state {
			t.Fatalf("transient monitoring failure in %q = %q, want unchanged", state, got)
		}

		got, err = Transition(state, EventPermanentMonitoringFailure)
		if err != nil {
			t.Fatalf("permanent monitoring failure in %q returned an error: %v", state, err)
		}
		if got != StateFailed {
			t.Fatalf("permanent monitoring failure in %q = %q, want %q", state, got, StateFailed)
		}
	}

	got, err := Transition(StateCollecting, EventCollectionFailed)
	if err != nil {
		t.Fatalf("collection failure returned an error: %v", err)
	}
	if got != StateCollecting {
		t.Fatalf("collection failure = %q, want collecting to remain visible", got)
	}
}
