package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSplitCheckOutput(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantOutput   string
		wantPerfData string
	}{
		{
			name:         "no pipe",
			input:        "OK - everything is fine",
			wantOutput:   "OK - everything is fine",
			wantPerfData: "",
		},
		{
			name:         "output and perfdata",
			input:        "OK - everything is fine | value=42",
			wantOutput:   "OK - everything is fine ",
			wantPerfData: "value=42",
		},
		{
			name:         "perfdata is trimmed",
			input:        "OK | value=42   \n",
			wantOutput:   "OK ",
			wantPerfData: "value=42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, perfData := splitCheckOutput([]byte(tt.input))

			if output != tt.wantOutput {
				t.Errorf("output = %q, want %q", output, tt.wantOutput)
			}
			if perfData != tt.wantPerfData {
				t.Errorf("perfData = %q, want %q", perfData, tt.wantPerfData)
			}
		})
	}
}

func baseCheck(command string, args ...string) Check {
	return Check{
		Name:    "test-check",
		Command: command,
		Args:    args,
		Timeout: 5 * time.Second,
	}
}

func TestRunCheck(t *testing.T) {
	ctx := context.Background()

	t.Run("ok exit code and perf data", func(t *testing.T) {
		result := runCheck(ctx, baseCheck("./test_checks/check-ok.sh"))

		if result.State != stateOK {
			t.Errorf("State = %v, want stateOK", result.State)
		}
		if !strings.Contains(result.Output, "everything is fine") {
			t.Errorf("Output = %q", result.Output)
		}
		if !strings.Contains(result.PerfData, "value=42") {
			t.Errorf("PerfData = %q", result.PerfData)
		}
		if result.Name != "test-check" {
			t.Errorf("Name = %q, want %q", result.Name, "test-check")
		}
		if result.Duration <= 0 {
			t.Errorf("Duration = %v, want > 0", result.Duration)
		}
	})

	t.Run("warning exit code", func(t *testing.T) {
		result := runCheck(ctx, baseCheck("./test_checks/check-warning-metric.sh"))

		if result.State != stateWarning {
			t.Errorf("State = %v, want stateWarning", result.State)
		}
	})

	t.Run("command not found is unknown", func(t *testing.T) {
		result := runCheck(ctx, baseCheck("./test_checks/does-not-exist.sh"))

		if result.State != stateUnknown {
			t.Errorf("State = %v, want stateUnknown", result.State)
		}
		if !strings.Contains(result.Output, "not found or not executable") {
			t.Errorf("Output = %q", result.Output)
		}
	})

	t.Run("non-executable file is unknown", func(t *testing.T) {
		result := runCheck(ctx, baseCheck("./test_checks/check-noexec.sh"))

		if result.State != stateUnknown {
			t.Errorf("State = %v, want stateUnknown", result.State)
		}
	})

	t.Run("passes arguments through", func(t *testing.T) {
		// check-args.sh reports a fixed value of 42; with warning=40 and
		// critical=50 that value is >= warning, so it should warn.
		result := runCheck(ctx, baseCheck("./test_checks/check-args.sh", "40", "50"))

		if result.State != stateWarning {
			t.Errorf("State = %v, want stateWarning, output: %q", result.State, result.Output)
		}
	})

	t.Run("missing required arguments is unknown", func(t *testing.T) {
		result := runCheck(ctx, baseCheck("./test_checks/check-args.sh"))

		if result.State != stateUnknown {
			t.Errorf("State = %v, want stateUnknown", result.State)
		}
	})

	t.Run("exit code outside 0-3 is unknown", func(t *testing.T) {
		check := baseCheck("/bin/sh", "-c", "echo 'weird exit'; exit 42")
		result := runCheck(ctx, check)

		if result.State != stateUnknown {
			t.Errorf("State = %v, want stateUnknown", result.State)
		}
		if !strings.Contains(result.Output, "unsupported status 42") {
			t.Errorf("Output = %q", result.Output)
		}
	})

	t.Run("timeout is unknown", func(t *testing.T) {
		check := baseCheck("./test_checks/check-slow.sh")
		check.Timeout = 200 * time.Millisecond

		start := time.Now()
		result := runCheck(ctx, check)
		elapsed := time.Since(start)

		if result.State != stateUnknown {
			t.Errorf("State = %v, want stateUnknown", result.State)
		}
		if !strings.Contains(result.Output, "timed out") {
			t.Errorf("Output = %q", result.Output)
		}
		if elapsed > 5*time.Second {
			t.Errorf("runCheck took %v, expected it to respect the short timeout", elapsed)
		}
	})

	t.Run("parent context cancellation is unknown", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()

		result := runCheck(cancelCtx, baseCheck("./test_checks/check-ok.sh"))

		if result.State != stateUnknown {
			t.Errorf("State = %v, want stateUnknown", result.State)
		}
	})

	t.Run("timestamps and duration are set", func(t *testing.T) {
		result := runCheck(ctx, baseCheck("./test_checks/check-ok.sh"))

		if result.StartedAt.IsZero() {
			t.Errorf("StartedAt should not be zero")
		}
		if result.FinishedAt.IsZero() {
			t.Errorf("FinishedAt should not be zero")
		}
		if result.FinishedAt.Before(result.StartedAt) {
			t.Errorf("FinishedAt %v is before StartedAt %v", result.FinishedAt, result.StartedAt)
		}
	})
}
