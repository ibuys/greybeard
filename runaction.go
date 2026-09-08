package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"syscall"
)

func runAction(
	parentCtx context.Context,
	action Action,
	input ActionInput,
) (string, error) {

	actionPath, err := exec.LookPath(action.Command)
	if err != nil {
		return "", fmt.Errorf(
			"action %q command not found or not executable: %s",
			action.Name,
			action.Command,
		)
	}

	payload, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf(
			"action %q unable to encode input: %w",
			action.Name,
			err,
		)
	}

	ctx, cancel := context.WithTimeout(parentCtx, action.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, actionPath, action.Args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}

	cmd.Stdin = bytes.NewReader(payload)

	output, err := cmd.CombinedOutput()

	switch ctx.Err() {
	case context.DeadlineExceeded:
		return string(output), fmt.Errorf(
			"action %q timed out after %s",
			action.Name,
			action.Timeout,
		)

	case context.Canceled:
		return string(output), fmt.Errorf(
			"action %q canceled",
			action.Name,
		)

	}

	if err == nil {
		return string(output), nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(output), fmt.Errorf(
			"action %q exited with status %d",
			action.Name,
			exitErr.ExitCode(),
		)
	}

	return string(output), fmt.Errorf(
		"action %q failed: %w",
		action.Name,
		err,
	)
}
