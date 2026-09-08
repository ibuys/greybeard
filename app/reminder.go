package main

import (
	"context"
	"time"
)

func runCriticalReminderSchedule(
	ctx context.Context,
	check Check,
	reminders chan<- string,
) {
	ticker := time.NewTicker(check.CriticalReminderInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			select {
			case reminders <- check.Name:
			case <-ctx.Done():
				return
			}

		case <-ctx.Done():
			return
		}
	}
}
