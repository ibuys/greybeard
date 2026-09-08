package main

import (
	"context"
	"fmt"
	"os"
	"time"
)

type Action struct {
	Name    string
	Command string
	Args    []string
	Timeout time.Duration
}

type CheckActions struct {
	OK       []string
	Warning  []string
	Critical []string
	Unknown  []string
}

type ActionTrigger string

const (
	actionTriggerTransition       ActionTrigger = "transition"
	actionTriggerCriticalReminder ActionTrigger = "critical_reminder"
)

type ActionInput struct {
	Trigger       ActionTrigger `json:"trigger"`
	CheckName     string        `json:"check_name"`
	Target        string        `json:"target"`
	PreviousState string        `json:"previous_state,omitempty"`
	State         string        `json:"state"`
	Output        string        `json:"output"`
}

func (actions CheckActions) namesForState(state State) []string {

	switch state {
	case stateOK:
		return actions.OK

	case stateWarning:
		return actions.Warning

	case stateCritical:
		return actions.Critical

	case stateUnknown:
		return actions.Unknown
	}

	return nil
}

func actionInputForTransition(
	transition StateTransition,
	result Result,
) ActionInput {

	input := ActionInput{
		Trigger:   actionTriggerTransition,
		CheckName: transition.Name,
		Target:    result.Target,
		State:     transition.Current.String(),
		Output:    result.Output,
	}

	if !transition.Initial {
		input.PreviousState = transition.Previous.String()
	}

	return input
}

// Take action

func runActions(
	ctx context.Context,
	check Check,
	actions map[string]Action,
	names []string,
	input ActionInput,
) {
	for _, name := range names {
		action := actions[name]

		output, err := runAction(ctx, action, input)
		if err != nil {
			fmt.Fprintf(
				os.Stderr,
				"%s, %s: action %q failed: %v\n",
				check.Name,
				check.Target,
				action.Name,
				err,
			)
			continue
		}

		fmt.Printf(
			"%s, %s: action %q completed\n",
			check.Name,
			check.Target,
			action.Name,
		)

		if output != "" {
			fmt.Printf(
				"%s, %s: action %q returned: %s",
				check.Name,
				check.Target,
				action.Name,
				output,
			)
		}

	}

}

func runTransitionActions(
	ctx context.Context,
	check Check,
	actions map[string]Action,
	transition StateTransition,
	result Result,
) {
	input := actionInputForTransition(transition, result)

	runActions(
		ctx,
		check,
		actions,
		check.Actions.namesForState(transition.Current),
		input,
	)
}

func runReminderActions(
	ctx context.Context,
	check Check,
	actions map[string]Action,
	output string,
) {
	input := actionInputForCriticalReminder(
		check.Name,
		check.Target,
		output,
	)

	runActions(
		ctx,
		check,
		actions,
		check.Actions.Critical,
		input,
	)
}

func actionInputForCriticalReminder(
	checkName string,
	checkTarget string,
	output string,
) ActionInput {
	return ActionInput{
		Trigger:   actionTriggerCriticalReminder,
		Target:    checkTarget,
		CheckName: checkName,
		State:     stateCritical.String(),
		Output:    output,
	}
}
