package release

import (
	"errors"
	"testing"
)

func mustTransition(t *testing.T, op Operation, event Event) Operation {
	t.Helper()
	next, err := Transition(op, event)
	if err != nil {
		t.Fatalf("%s + %s: %v", op.State, event, err)
	}
	return next
}

func mustIllegal(t *testing.T, op Operation, event Event) {
	t.Helper()
	if _, err := Transition(op, event); !errors.Is(err, ErrIllegalTransition) {
		t.Fatalf("%s + %s: expected illegal transition, got %v", op.State, event, err)
	}
}

func TestHappyPathToReady(t *testing.T) {
	op := Operation{State: Pending}
	op = mustTransition(t, op, EventClaim)
	if op.State != Applying {
		t.Fatal(op.State)
	}
	op = mustTransition(t, op, EventReadbackMatch)
	if op.State != Ready {
		t.Fatal(op.State)
	}
}

func TestMismatchRetriesThenFails(t *testing.T) {
	op := Operation{State: Applying}
	for i := 1; i < MaxAttempts; i++ {
		op = mustTransition(t, op, EventReadbackMismatch)
		if op.State != Applying || op.Attempts != i {
			t.Fatalf("attempt %d: %+v", i, op)
		}
	}
	op = mustTransition(t, op, EventReadbackMismatch)
	if op.State != Failed || op.Attempts != MaxAttempts {
		t.Fatalf("expected failure at max attempts: %+v", op)
	}
}

func TestFailedRetryResetsAttempts(t *testing.T) {
	op := Operation{State: Failed, Attempts: MaxAttempts}
	op = mustTransition(t, op, EventRetry)
	if op.State != Pending || op.Attempts != 0 {
		t.Fatalf("%+v", op)
	}
}

func TestClaimExpiryRecoversApplying(t *testing.T) {
	// Controller crash mid-apply: the operation returns to Pending and can be
	// claimed again without losing its attempt count.
	op := Operation{State: Applying, Attempts: 2}
	op = mustTransition(t, op, EventExpire)
	if op.State != Pending || op.Attempts != 2 {
		t.Fatalf("%+v", op)
	}
	op = mustTransition(t, op, EventClaim)
	if op.State != Applying {
		t.Fatal(op.State)
	}
}

func TestReadyDriftAndReconcile(t *testing.T) {
	op := Operation{State: Ready}
	op = mustTransition(t, op, EventReadbackMismatch)
	if op.State != Drifted {
		t.Fatal(op.State)
	}
	op = mustTransition(t, op, EventReconcile)
	if op.State != Applying || op.Attempts != 0 {
		t.Fatalf("%+v", op)
	}
	op = mustTransition(t, op, EventReadbackMatch)
	if op.State != Ready {
		t.Fatal(op.State)
	}
}

func TestIllegalTransitions(t *testing.T) {
	cases := []struct {
		state State
		event Event
	}{
		{Pending, EventReadbackMatch},
		{Pending, EventRetry},
		{Applying, EventRetry},
		{Ready, EventClaim},
		{Ready, EventRetry},
		{Failed, EventClaim},
		{Failed, EventReadbackMatch},
		{Drifted, EventReadbackMatch},
	}
	for _, c := range cases {
		mustIllegal(t, Operation{State: c.state}, c.event)
	}
}

func TestValidState(t *testing.T) {
	for _, s := range []State{Pending, Applying, Ready, Failed, Drifted} {
		if !ValidState(s) {
			t.Fatalf("%s must be valid", s)
		}
	}
	if ValidState("Bogus") {
		t.Fatal("bogus state accepted")
	}
}
