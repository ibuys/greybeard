package main

import "time"

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
