package main

import (
	"testing"
)

func TestCheckActionsNamesForState(t *testing.T) {

	actions := CheckActions{
		OK:       []string{"ok-action"},
		Warning:  []string{"warning-action"},
		Critical: []string{"critical-action"},
		Unknown:  []string{"unknown-action"},
	}

	tests := []struct {
		name  string
		state State
		want  string
	}{
		{"ok", stateOK, "ok-action"},
		{"warning", stateWarning, "warning-action"},
		{"critical", stateCritical, "critical-action"},
		{"unknown", stateUnknown, "unknown-action"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			names := actions.namesForState(test.state)

			if len(names) != 1 {
				t.Fatalf("got %d actions, want 1", len(names))
			}

			if names[0] != test.want {
				t.Errorf("action = %q, want %q", names[0], test.want)
			}
		})
	}
}

func TestActionInputForTransition(t *testing.T) {

	t.Run("state change", func(t *testing.T) {

		transition := StateTransition{
			Name:     "apache",
			Previous: stateOK,
			Current:  stateCritical,
		}

		result := Result{
			Output: "CRITICAL - Apache is down\n",
		}

		got := actionInputForTransition(transition, result)

		want := ActionInput{
			Trigger:       actionTriggerTransition,
			CheckName:     "apache",
			PreviousState: "OK",
			State:         "CRITICAL",
			Output:        "CRITICAL - Apache is down\n",
		}

		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("initial state", func(t *testing.T) {

		transition := StateTransition{
			Name:    "apache",
			Current: stateCritical,
			Initial: true,
		}

		result := Result{
			Output: "CRITICAL - Apache is down\n",
		}

		got := actionInputForTransition(transition, result)

		want := ActionInput{
			Trigger:   actionTriggerTransition,
			CheckName: "apache",
			State:     "CRITICAL",
			Output:    "CRITICAL - Apache is down\n",
		}

		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
}

func TestActionInputForCriticalReminder(t *testing.T) {

	got := actionInputForCriticalReminder(
		"apache",
		"CRITICAL - Apache is down\n",
	)

	want := ActionInput{
		Trigger:   actionTriggerCriticalReminder,
		CheckName: "apache",
		State:     "CRITICAL",
		Output:    "CRITICAL - Apache is down\n",
	}

	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
