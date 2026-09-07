package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {

	var configFile string

	flag.StringVar(
		&configFile,
		"c",
		"",
		"configuration file",
	)

	flag.StringVar(
		&configFile,
		"config-file",
		"",
		"configuration file",
	)

	flag.Parse()

	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: greybeard [-c config-file]")
		os.Exit(int(stateUnknown))
	}

	if configFile == "" {
		var err error

		configFile, err = findConfigFile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "UNKNOWN - %v\n", err)
			os.Exit(int(stateUnknown))
		}
	}

	config, err := loadConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "UNKNOWN - %v\n", err)
		os.Exit(int(stateUnknown))
	}

	checks := config.Checks

	databaseURL := os.Getenv("GREYBEARD_DATABASE_URL")

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	defer stop()

	var wg sync.WaitGroup

	results := make(chan Result)

	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "UNKNOWN - GREYBEARD_DATABASE_URL is required")
		os.Exit(int(stateUnknown))
	}

	resultStore, err := NewPostgresResultStore(ctx, databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "UNKNOWN - unable to initialize result store: %v\n", err)
		os.Exit(int(stateUnknown))
	}

	names := make([]string, 0, len(checks))
	attempts := make(map[string]int, len(checks))
	checksByName := make(map[string]Check, len(checks))

	for _, check := range checks {
		names = append(names, check.Name)
		attempts[check.Name] = check.Attempts
		checksByName[check.Name] = check
	}

	states, err := resultStore.LoadStates(ctx, names)
	if err != nil {
		fmt.Fprintf(os.Stderr, "UNKNOWN - unable to load persisted state: %v\n", err)
		os.Exit(int(stateUnknown))
	}

	stateTracker := NewStateTracker(states)

	for _, check := range checks {

		wg.Add(1)

		go func(check Check) {
			defer wg.Done()

			runScheduledCheck(
				ctx,
				check,
				config.StartupSpread,
				results,
			)

		}(check)
	}

mainLoop:
	for {
		select {
		case result := <-results:
			if ctx.Err() != nil {
				break mainLoop
			}

			metrics, err := parsePerfData(result.PerfData)
			if err != nil {
				fmt.Fprintf(
					os.Stderr,
					"%s: invalid performance data: %v\n",
					result.Name,
					err,
				)
				result.PerfDataError = err.Error()
			} else {
				result.Metrics = metrics
			}

			decision := stateTracker.Observe(
				result,
				attempts[result.Name],
			)

			err = resultStore.Record(
				ctx,
				result,
				decision.Transition != nil,
			)

			if err != nil {
				fmt.Fprintf(
					os.Stderr,
					"%s: unable to persist result: %v\n",
					result.Name,
					err,
				)

				stop()
				wg.Wait()
				os.Exit(int(stateUnknown))
			}

			if decision.Transition != nil {

				transition := *decision.Transition

				handleTransition(transition, result)

				check := checksByName[result.Name]

				wg.Add(1)

				go func() {
					defer wg.Done()

					runTransitionActions(
						ctx,
						check,
						config.Actions,
						transition,
						result,
					)
				}()

			} else if decision.Candidate != nil {
				fmt.Printf(
					"%s: candidate %s %d/%d: %s",
					result.Name,
					decision.Candidate.State,
					decision.Candidate.Count,
					attempts[result.Name],
					result.Output,
				)
			} else {
				fmt.Printf("%s: %s", result.Name, result.Output)
			}

			for _, metric := range result.Metrics {
				fmt.Printf(
					"DEBUG metric: name=%q unit=%q warning=%q critical=%q",
					metric.Name,
					metric.Unit,
					metric.Warning,
					metric.Critical,
				)

				if metric.Value == nil {
					fmt.Print(" value=U")
				} else {
					fmt.Printf(" value=%g", *metric.Value)
				}

				if metric.Minimum != nil {
					fmt.Printf(" min=%g", *metric.Minimum)
				}

				if metric.Maximum != nil {
					fmt.Printf(" max=%g", *metric.Maximum)
				}
				fmt.Println()
			}

		case <-ctx.Done():
			break mainLoop
		}
	}

	fmt.Println("Greybeard shutting down")
	wg.Wait()

	resultStore.pool.Close()

	fmt.Println("Greybeard stopped")
	return

}
