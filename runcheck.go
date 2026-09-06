package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func splitCheckOutput(output []byte) (string, string) {
	checkOutput, perfData, found := strings.Cut(string(output), "|")

	if !found {
		return string(output), ""
	}

	return checkOutput, strings.TrimSpace(perfData)
}

func runCheck(parentCtx context.Context, check Check) (result Result) {

	result = Result{
		Name:      check.Name,
		State:     stateUnknown,
		StartedAt: time.Now(),
	}

	defer func() {
		if result.FinishedAt.IsZero() {
			result.FinishedAt = time.Now()
		}

		result.Duration = result.FinishedAt.Sub(result.StartedAt)
	}()

	checkPath, err := exec.LookPath(check.Command)
	if err != nil {
		result.Output = fmt.Sprintf("UNKNOWN - check not found or not executable: %s\n", check.Command)
		return result
	}

	ctx, cancel := context.WithTimeout(parentCtx, check.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, checkPath, check.Args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}

	output, err := cmd.CombinedOutput()
	result.FinishedAt = time.Now()

	checkOutput, perfData := splitCheckOutput(output)

	switch ctx.Err() {
	case context.DeadlineExceeded:
		result.Output = fmt.Sprintf("UNKNOWN - check timed out after %s\n", check.Timeout)
		return result

	case context.Canceled:
		result.Output = "UNKNOWN - check canceled\n"
		return result
	}

	if err == nil {
		result.State = stateOK
		result.Output = checkOutput
		result.PerfData = perfData
		return result
	}

	if exitErr, isExitError := err.(*exec.ExitError); isExitError {
		code := exitErr.ExitCode()

		if code >= int(stateOK) && code <= int(stateUnknown) {
			result.State = State(code)
			result.Output = checkOutput
			result.PerfData = perfData
			return result
		}

		result.Output = fmt.Sprintf("%s UNKNOWN - check exited with unsupported status %d\n", checkOutput, code)
		result.PerfData = perfData
		return result
	}

	result.Output = fmt.Sprintf("%sUNKNOWN - unable to run check %v\n", string(output), err)
	return result
}
