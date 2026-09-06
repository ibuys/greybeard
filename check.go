package main

import (
	"time"
)

type State int

const (
	stateOK State = iota
	stateWarning
	stateCritical
	stateUnknown
)

type Check struct {
	Name                     string
	Command                  string
	Args                     []string
	Timeout                  time.Duration
	Interval                 time.Duration
	Attempts                 int
	CriticalReminderInterval time.Duration
	Actions                  CheckActions
}

type Result struct {
	Name          string
	State         State
	Output        string
	PerfData      string
	PerfDataError string
	Metrics       []Metric
	StartedAt     time.Time
	FinishedAt    time.Time
	Duration      time.Duration
}
