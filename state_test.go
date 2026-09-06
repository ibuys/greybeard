package main

import (
	"testing"
	"time"
)

func TestStateTrackerInitialObservation(t *testing.T) {
	tracker := NewStateTracker(nil)
	now := time.Now()

	result := Result{
		Name:       "check-a",
		State:      stateOK,
		Output:     "OK",
		FinishedAt: now,
	}

	decision := tracker.Observe(result, 3)

	if decision.Transition == nil {
		t.Fatal("expected initial observation to establish confirmed state")
	}

	if decision.Transition == nil {
		t.Fatal("expected initial transition")
	}

	if !decision.Transition.Initial {
		t.Errorf("expected Initial = true")
	}
}

func TestStateTrackerConfirmsAfterThreeAttempts(t *testing.T) {
	now := time.Now()

	initial := map[string]State{
		"check-a": stateOK,
	}

	tracker := NewStateTracker(initial)

	result := Result{
		Name:       "check-a",
		State:      stateCritical,
		Output:     "CRITICAL",
		FinishedAt: now.Add(time.Minute),
	}

	decision := tracker.Observe(result, 3)

	if decision.Transition != nil {
		t.Fatal("did not expect state update after attempt 1")
	}

	decision = tracker.Observe(result, 3)

	if decision.Transition != nil {
		t.Fatal("did not expect state update after attempt 2")
	}

	decision = tracker.Observe(result, 3)

	if decision.Transition == nil {
		t.Fatal("expected state update after attempt 3")
	}

	if decision.Transition == nil {
		t.Fatal("expected transition after attempt 3")
	}

	if decision.Transition.Previous != stateOK {
		t.Errorf("Previous = %v, want OK", decision.Transition.Previous)
	}

	if decision.Transition.Current != stateCritical {
		t.Errorf("Current = %v, want CRITICAL", decision.Transition.Current)
	}

	if decision.Transition.Initial {
		t.Fatal("expected Initial = false")
	}
}

func TestStateTrackerConfirmedStateClearsCandidate(t *testing.T) {
	initial := map[string]State{
		"check-a": stateOK,
	}

	tracker := NewStateTracker(initial)

	tracker.Observe(
		Result{Name: "check-a", State: stateCritical},
		3,
	)

	decision := tracker.Observe(
		Result{Name: "check-a", State: stateOK},
		3,
	)
	if decision.Transition != nil {
		t.Fatal("did not expect transition")
	}

	if decision.Candidate != nil {
		t.Fatal("did not expect candidate")
	}
	if decision.Transition != nil {
		t.Fatal("did not expect state update")
	}

	if _, exists := tracker.candidates["check-a"]; exists {
		t.Fatal("expected candidate to be cleared")
	}
}

func TestStateTrackerDifferentCandidateResetsAttempts(t *testing.T) {

	initial := map[string]State{
		"check-a": stateOK,
	}

	tracker := NewStateTracker(initial)

	tracker.Observe(
		Result{Name: "check-a", State: stateCritical},
		3,
	)

	tracker.Observe(
		Result{Name: "check-a", State: stateCritical},
		3,
	)

	tracker.Observe(
		Result{Name: "check-a", State: stateWarning},
		3,
	)

	candidate, exists := tracker.candidates["check-a"]
	if !exists {
		t.Fatal("expected candidate")
	}

	if candidate.State != stateWarning {
		t.Errorf("candidate State = %v, want WARNING", candidate.State)
	}

	if candidate.Count != 1 {
		t.Errorf("candidate Attempts = %d, want 1", candidate.Count)
	}
}

func TestStateTrackerAttemptsOneConfirmsImmediately(t *testing.T) {
	initial := map[string]State{
		"check-a": stateOK,
	}

	tracker := NewStateTracker(initial)

	decision := tracker.Observe(
		Result{
			Name:  "check-a",
			State: stateCritical,
		},
		1,
	)

	if decision.Transition == nil {
		t.Fatal("expected immediate state update")
	}
}

func TestStateTrackerRecoveryAlsoRequiresConfirmation(t *testing.T) {
	initial := map[string]State{
		"check-a": stateCritical,
	}

	tracker := NewStateTracker(initial)

	first := tracker.Observe(
		Result{Name: "check-a", State: stateOK},
		3,
	)

	second := tracker.Observe(
		Result{Name: "check-a", State: stateOK},
		3,
	)

	third := tracker.Observe(
		Result{Name: "check-a", State: stateOK},
		3,
	)

	if first.Transition != nil {
		t.Fatal("did not expect recovery after attempt 1")
	}

	if second.Transition != nil {
		t.Fatal("did not expect recovery after attempt 2")
	}

	if third.Transition == nil {
		t.Fatal("expected recovery after attempt 3")
	}

	if third.Transition.Current != stateOK {
		t.Errorf("Current = %v, want OK", third.Transition.Current)
	}
}

func TestStateTrackerRestartDiscardsCandidate(t *testing.T) {
	initial := map[string]State{
		"check-a": stateOK,
	}

	tracker := NewStateTracker(initial)

	// Two CRITICAL observations: candidate is now 2/3.
	tracker.Observe(
		Result{Name: "check-a", State: stateCritical},
		3,
	)

	tracker.Observe(
		Result{Name: "check-a", State: stateCritical},
		3,
	)

	// Simulate Greybeard restarting. Only confirmed state is loaded.
	tracker = NewStateTracker(initial)

	decision := tracker.Observe(
		Result{Name: "check-a", State: stateCritical},
		3,
	)

	if decision.Transition != nil {
		t.Fatal("did not expect state change after first post-restart observation")
	}

	candidate := tracker.candidates["check-a"]

	if candidate.Count != 1 {
		t.Errorf("candidate Attempts = %d, want 1", candidate.Count)
	}
}
