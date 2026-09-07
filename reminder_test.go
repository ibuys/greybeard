package main

import (
	"context"
	"testing"
	"time"
)

func TestRunCriticalReminderSchedule(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	check := Check{
		Name:                     "apache",
		CriticalReminderInterval: 50 * time.Millisecond,
	}

	reminders := make(chan string)
	done := make(chan struct{})

	go func() {
		defer close(done)

		runCriticalReminderSchedule(
			ctx,
			check,
			reminders,
		)
	}()

	// The first reminder should not happen immediately.
	select {
	case name := <-reminders:
		t.Fatalf("unexpected early reminder for %q", name)
	case <-time.After(10 * time.Millisecond):
	}

	// Then one should arrive after the configured interval.
	select {
	case name := <-reminders:
		if name != "apache" {
			t.Errorf("got reminder for %q, want apache", name)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for reminder")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("reminder schedule did not stop after cancellation")
	}
}
