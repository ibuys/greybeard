package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunActionSuccess(t *testing.T) {

	action := Action{
		Name:    "success",
		Command: "/bin/echo",
		Args:    []string{"hello"},
		Timeout: time.Second,
	}

	output, err := runAction(context.Background(), action, ActionInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output != "hello\n" {
		t.Errorf("output = %q, want %q", output, "hello\n")
	}
}

func TestRunActionFailure(t *testing.T) {

	action := Action{
		Name:    "failure",
		Command: "/bin/sh",
		Args:    []string{"-c", "echo failed; exit 2"},
		Timeout: time.Second,
	}

	output, err := runAction(context.Background(), action, ActionInput{})

	if err == nil {
		t.Fatal("expected error, got none")
	}

	if output != "failed\n" {
		t.Errorf("output = %q, want %q", output, "failed\n")
	}
}

func TestRunActionTimeout(t *testing.T) {

	action := Action{
		Name:    "timeout",
		Command: "/bin/sh",
		Args:    []string{"-c", "sleep 10"},
		Timeout: 100 * time.Millisecond,
	}

	_, err := runAction(context.Background(), action, ActionInput{})

	if err == nil {
		t.Fatal("expected timeout error, got none")
	}

	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error = %q, want timeout error", err)
	}
}

func TestRunActionCanceled(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())

	action := Action{
		Name:    "canceled",
		Command: "/bin/sh",
		Args:    []string{"-c", "sleep 10"},
		Timeout: time.Second,
	}

	cancel()

	_, err := runAction(ctx, action, ActionInput{})

	if err == nil {
		t.Fatal("expected cancellation error, got none")
	}

	if !strings.Contains(err.Error(), "canceled") {
		t.Errorf("error = %q, want cancellation error", err)
	}
}

func TestRunActionCommandNotFound(t *testing.T) {

	action := Action{
		Name:    "missing",
		Command: "/definitely/does/not/exist",
		Timeout: time.Second,
	}

	_, err := runAction(context.Background(), action, ActionInput{})

	if err == nil {
		t.Fatal("expected error, got none")
	}

	if !strings.Contains(err.Error(), "not found or not executable") {
		t.Errorf("error = %q, want command-not-found error", err)
	}
}

func TestRunActionReceivesJSONInput(t *testing.T) {

	action := Action{
		Name:    "cat",
		Command: "/bin/cat",
		Timeout: time.Second,
	}

	input := ActionInput{
		Trigger:       actionTriggerTransition,
		CheckName:     "apache",
		PreviousState: "OK",
		State:         "CRITICAL",
		Output:        "CRITICAL - Apache is not responding\n",
	}

	output, err := runAction(context.Background(), action, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got ActionInput

	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("unable to decode action input: %v", err)
	}

	if got != input {
		t.Errorf("got %+v, want %+v", got, input)
	}
}

func TestRunTransitionActionsSelectsCurrentState(t *testing.T) {

	dir := t.TempDir()
	criticalFile := filepath.Join(dir, "critical.json")
	okFile := filepath.Join(dir, "ok.json")

	check := Check{
		Name: "apache",
		Actions: CheckActions{
			OK:       []string{"ok-action"},
			Critical: []string{"critical-action"},
		},
	}

	actions := map[string]Action{
		"ok-action": {
			Name:    "ok-action",
			Command: "/bin/sh",
			Args: []string{
				"-c",
				`cat > "$1"`,
				"sh",
				okFile,
			},
			Timeout: time.Second,
		},
		"critical-action": {
			Name:    "critical-action",
			Command: "/bin/sh",
			Args: []string{
				"-c",
				`cat > "$1"`,
				"sh",
				criticalFile,
			},
			Timeout: time.Second,
		},
	}

	transition := StateTransition{
		Name:     "apache",
		Previous: stateOK,
		Current:  stateCritical,
	}

	result := Result{
		Output: "CRITICAL - Apache is down\n",
	}

	runTransitionActions(
		context.Background(),
		check,
		actions,
		transition,
		result,
	)

	data, err := os.ReadFile(criticalFile)
	if err != nil {
		t.Fatalf("unable to read critical action input: %v", err)
	}

	var got ActionInput

	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unable to decode critical action input: %v", err)
	}

	want := actionInputForTransition(transition, result)

	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}

	if _, err := os.Stat(okFile); !os.IsNotExist(err) {
		t.Errorf("OK action ran unexpectedly")
	}
}

func TestRunTransitionActionsContinuesAfterFailure(t *testing.T) {

	dir := t.TempDir()
	successFile := filepath.Join(dir, "success.json")

	check := Check{
		Name: "apache",
		Actions: CheckActions{
			Critical: []string{
				"failing-action",
				"successful-action",
			},
		},
	}

	actions := map[string]Action{
		"failing-action": {
			Name:    "failing-action",
			Command: "/bin/sh",
			Args:    []string{"-c", "exit 1"},
			Timeout: time.Second,
		},
		"successful-action": {
			Name:    "successful-action",
			Command: "/bin/sh",
			Args: []string{
				"-c",
				`cat > "$1"`,
				"sh",
				successFile,
			},
			Timeout: time.Second,
		},
	}

	transition := StateTransition{
		Name:     "apache",
		Previous: stateOK,
		Current:  stateCritical,
	}

	result := Result{
		Output: "CRITICAL - Apache is down\n",
	}

	runTransitionActions(
		context.Background(),
		check,
		actions,
		transition,
		result,
	)

	data, err := os.ReadFile(successFile)
	if err != nil {
		t.Fatalf("successful action did not run: %v", err)
	}

	var got ActionInput

	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unable to decode successful action input: %v", err)
	}

	want := actionInputForTransition(transition, result)

	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
