package main

import (
	"context"
	"math/rand"
	"time"
)

func sendResult(
	ctx context.Context,
	results chan<- Result,
	result Result,
) bool {
	select {
	case results <- result:
		return true
	case <-ctx.Done():
		return false
	}
}

func runScheduledCheck(
	ctx context.Context,
	check Check,
	startupSpread time.Duration,
	results chan<- Result,
) {
	startupDelay := time.Duration(0)

	if startupSpread > 0 {
		startupDelay = time.Duration(
			rand.Int63n(int64(startupSpread)),
		)
	}

	if startupDelay > 0 {
		timer := time.NewTimer(startupDelay)

		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return
		}
	}

	ticker := time.NewTicker(check.Interval)
	defer ticker.Stop()

	result := runCheck(ctx, check)

	if !sendResult(ctx, results, result) {
		return
	}

	for {
		select {
		case <-ticker.C:
			result := runCheck(ctx, check)

			if !sendResult(ctx, results, result) {
				return
			}

		case <-ctx.Done():
			return
		}
	}

}
