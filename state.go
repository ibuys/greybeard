package main

import (
	"fmt"
	"time"
)

type CheckState struct {
	State     State
	Output    string
	ChangedAt time.Time
}

type StateTransition struct {
	Name     string
	Previous State
	Current  State
	Initial  bool
}

type StateCandidate struct {
	State State
	Count int
}

type StateTracker struct {
	confirmed  map[string]State
	candidates map[string]StateCandidate
}

type StateDecision struct {
	Transition *StateTransition
	Candidate  *StateCandidate
}

func (state State) String() string {
	switch state {
	case stateOK:
		return "OK"
	case stateWarning:
		return "WARNING"
	case stateCritical:
		return "CRITICAL"
	case stateUnknown:
		return "UNKNOWN"
	default:
		return "INVALID"
	}
}

func handleTransition(transition StateTransition, result Result) {
	if transition.Initial {
		fmt.Printf(
			"%s: initial state %s: %s",
			transition.Name,
			transition.Current,
			result.Output,
		)
	} else {
		fmt.Printf(
			"%s: state changed %s -> %s: %s",
			transition.Name,
			transition.Previous,
			transition.Current,
			result.Output,
		)
	}
}

func NewStateTracker(states map[string]State) *StateTracker {
	confirmed := make(map[string]State, len(states))

	for name, state := range states {
		confirmed[name] = state
	}

	return &StateTracker{
		confirmed:  confirmed,
		candidates: make(map[string]StateCandidate),
	}
}

func (tracker *StateTracker) Observe(
	result Result,
	threshold int,
) StateDecision {
	confirmed, exists := tracker.confirmed[result.Name]

	// First observation establishes confirmed state immediately.
	if !exists {

		transition := StateTransition{
			Name:    result.Name,
			Current: result.State,
			Initial: true,
		}

		tracker.confirmed[result.Name] = result.State
		delete(tracker.candidates, result.Name)

		return StateDecision{
			Transition: &transition,
		}
	}

	// Observation agrees with confirmed state.
	// Any pending candidate no longer consecutive.

	if result.State == confirmed {
		delete(tracker.candidates, result.Name)
		return StateDecision{}
	}

	candidate, exists := tracker.candidates[result.Name]

	// No candidate exists, start over
	if !exists || candidate.State != result.State {
		candidate = StateCandidate{
			State: result.State,
			Count: 1,
		}
	} else {
		candidate.Count++
	}

	// Not enough consecutive observations yet.
	if candidate.Count < threshold {
		tracker.candidates[result.Name] = candidate
		return StateDecision{
			Candidate: &candidate,
		}
	}

	// Candidate reached the confirmation threshold
	transition := StateTransition{
		Name:     result.Name,
		Previous: confirmed,
		Current:  result.State,
	}

	tracker.confirmed[result.Name] = result.State
	delete(tracker.candidates, result.Name)

	return StateDecision{
		Transition: &transition,
	}
}
