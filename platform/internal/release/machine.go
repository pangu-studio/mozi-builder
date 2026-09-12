// Package release implements the phase-4 release state machine: durable
// operations with idempotent execution, read-back verification, drift
// detection, and crash recovery via claim expiry. See docs/v2/deployment.md.
package release

import (
	"errors"
	"fmt"
)

// State is the lifecycle of a release operation.
type State string

const (
	Pending  State = "Pending"
	Applying State = "Applying"
	Ready    State = "Ready"
	Failed   State = "Failed"
	Drifted  State = "Drifted"
)

// Event drives a state transition.
type Event string

const (
	EventClaim            Event = "claim"             // controller picks up the operation
	EventReadbackMatch    Event = "readback_match"    // observed state equals desired
	EventReadbackMismatch Event = "readback_mismatch" // observed state differs
	EventFatal            Event = "fatal"             // unrecoverable execution error
	EventRetry            Event = "retry"             // operator requeues a failed op
	EventExpire           Event = "expire"            // claim lease expired (crash/timeout)
	EventReconcile        Event = "reconcile"         // operator re-applies desired state
)

// ErrIllegalTransition is returned for events not allowed in the current state.
var ErrIllegalTransition = errors.New("illegal release state transition")

// MaxAttempts bounds read-back retries before an operation fails.
const MaxAttempts = 5

// Operation is the state-machine view of a release_operations row.
type Operation struct {
	State    State
	Attempts int
}

// Transition applies an event and returns the new operation state. Read-back
// mismatches during Applying retry until MaxAttempts, then fail. Claim expiry
// returns Applying operations to Pending so another controller can resume
// execution — adapters must be idempotent (upsert semantics).
func Transition(op Operation, event Event) (Operation, error) {
	switch op.State {
	case Pending:
		if event == EventClaim {
			return Operation{State: Applying, Attempts: op.Attempts}, nil
		}
	case Applying:
		switch event {
		case EventReadbackMatch:
			return Operation{State: Ready, Attempts: op.Attempts}, nil
		case EventReadbackMismatch:
			if op.Attempts+1 >= MaxAttempts {
				return Operation{State: Failed, Attempts: op.Attempts + 1}, nil
			}
			return Operation{State: Applying, Attempts: op.Attempts + 1}, nil
		case EventFatal:
			return Operation{State: Failed, Attempts: op.Attempts}, nil
		case EventExpire:
			return Operation{State: Pending, Attempts: op.Attempts}, nil
		}
	case Ready:
		if event == EventReadbackMismatch {
			return Operation{State: Drifted, Attempts: op.Attempts}, nil
		}
	case Failed:
		if event == EventRetry {
			return Operation{State: Pending, Attempts: 0}, nil
		}
	case Drifted:
		if event == EventReconcile {
			return Operation{State: Applying, Attempts: 0}, nil
		}
	}
	return op, fmt.Errorf("%w: %s + %s", ErrIllegalTransition, op.State, event)
}

// ValidState reports whether s is a persisted state value.
func ValidState(s State) bool {
	switch s {
	case Pending, Applying, Ready, Failed, Drifted:
		return true
	}
	return false
}
