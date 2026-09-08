package main

import (
	"context"
	"testing"
	"time"
)

func TestSendResult(t *testing.T) {
	t.Run("delivers result when channel has room", func(t *testing.T) {
		ctx := context.Background()
		results := make(chan Result, 1)

		ok := sendResult(ctx, results, Result{Name: "check-a"})
		if !ok {
			t.Fatalf("expected sendResult to return true")
		}

		select {
		case result := <-results:
			if result.Name != "check-a" {
				t.Errorf("Name = %q, want %q", result.Name, "check-a")
			}
		default:
			t.Fatalf("expected a result to be available on the channel")
		}
	})

	t.Run("returns false when context is already done", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Unbuffered channel with no reader: the only way sendResult can
		// return is by observing ctx.Done().
		results := make(chan Result)

		ok := sendResult(ctx, results, Result{Name: "check-a"})
		if ok {
			t.Fatalf("expected sendResult to return false for a canceled context")
		}
	})
}

func TestRunScheduledCheck(t *testing.T) {
	t.Run("runs immediately and then on each tick", func(t *testing.T) {
		check := Check{
			Name:     "fast-check",
			Command:  "./test_checks/check-ok.sh",
			Timeout:  time.Second,
			Interval: 30 * time.Millisecond,
		}

		results := make(chan Result)
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			runScheduledCheck(ctx, check, 0, results)
			close(done)
		}()

		received := 0
		timeout := time.After(2 * time.Second)

		for received < 2 {
			select {
			case result := <-results:
				if result.Name != "fast-check" {
					t.Errorf("Name = %q, want %q", result.Name, "fast-check")
				}
				received++
			case <-timeout:
				t.Fatalf("timed out waiting for scheduled results, got %d", received)
			}
		}

		cancel()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("runScheduledCheck did not return after context cancellation")
		}
	})

	t.Run("returns without running when context is canceled during startup delay", func(t *testing.T) {
		check := Check{
			Name:     "delayed-check",
			Command:  "./test_checks/check-ok.sh",
			Timeout:  time.Second,
			Interval: time.Second,
		}

		results := make(chan Result, 1)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		done := make(chan struct{})
		go func() {
			runScheduledCheck(ctx, check, time.Hour, results)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("runScheduledCheck did not return promptly for a canceled context")
		}

		select {
		case result := <-results:
			t.Fatalf("did not expect a result, got %+v", result)
		default:
		}
	})
}
