package main

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestPostgresResultStoreIsCriticalAcknowledged(t *testing.T) {
	databaseURL := os.Getenv("GREYBEARD_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GREYBEARD_DATABASE_URL is not set")
	}

	ctx := context.Background()

	store, err := NewPostgresResultStore(ctx, databaseURL)
	if err != nil {
		t.Fatalf("unable to create result store: %v", err)
	}

	name := "test-acknowledgement"

	defer func() {
		_, _ = store.pool.Exec(
			ctx,
			`DELETE FROM greybeard.states WHERE check_name = $1`,
			name,
		)
	}()

	t.Run("no row", func(t *testing.T) {
		acknowledged, err := store.IsCriticalAcknowledged(ctx, name)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if acknowledged {
			t.Fatal("expected false for missing state")
		}
	})

	t.Run("critical not acknowledged", func(t *testing.T) {
		_, err := store.pool.Exec(
			ctx,
			`
				INSERT INTO greybeard.states (
					check_name,
					state,
					output,
					changed_at
				)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (check_name)
				DO UPDATE SET
					state = EXCLUDED.state,
					output = EXCLUDED.output,
					changed_at = EXCLUDED.changed_at,
					acknowledged_at = NULL
			`,
			name,
			int16(stateCritical),
			"CRITICAL - test",
			time.Now(),
		)

		if err != nil {
			t.Fatalf("unable to create test state: %v", err)
		}

		acknowledged, err := store.IsCriticalAcknowledged(ctx, name)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if acknowledged {
			t.Fatal("expected false for unacknowledged CRITICAL state")
		}
	})

	t.Run("critical acknowledged", func(t *testing.T) {
		_, err := store.pool.Exec(
			ctx,
			`
				UPDATE greybeard.states
				SET acknowledged_at = now()
				WHERE check_name = $1
			`,
			name,
		)

		if err != nil {
			t.Fatalf("unable to acknowledge test state: %v", err)
		}

		acknowledged, err := store.IsCriticalAcknowledged(ctx, name)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !acknowledged {
			t.Fatal("expected true for acknowledged CRITICAL state")
		}
	})

	t.Run("non-critical acknowledged", func(t *testing.T) {
		_, err := store.pool.Exec(
			ctx,
			`
				UPDATE greybeard.states
				SET state = $2,
				    acknowledged_at = now()
				WHERE check_name = $1
			`,
			name,
			int16(stateOK),
		)

		if err != nil {
			t.Fatalf("unable to update test state: %v", err)
		}

		acknowledged, err := store.IsCriticalAcknowledged(ctx, name)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if acknowledged {
			t.Fatal("expected false for acknowledged non-CRITICAL state")
		}
	})
}

func TestPostgresResultStoreRecordClearsAcknowledgement(t *testing.T) {
	databaseURL := os.Getenv("GREYBEARD_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GREYBEARD_DATABASE_URL is not set")
	}

	ctx := context.Background()

	store, err := NewPostgresResultStore(ctx, databaseURL)
	if err != nil {
		t.Fatalf("unable to create result store: %v", err)
	}

	name := "test-acknowledgement-clear"

	defer func() {
		_, _ = store.pool.Exec(
			ctx,
			`
				DELETE FROM greybeard.metrics
				WHERE result_id IN (
					SELECT id
					FROM greybeard.results
					WHERE check_name = $1
				)
			`,
			name,
		)

		_, _ = store.pool.Exec(
			ctx,
			`DELETE FROM greybeard.results WHERE check_name = $1`,
			name,
		)

		_, _ = store.pool.Exec(
			ctx,
			`DELETE FROM greybeard.states WHERE check_name = $1`,
			name,
		)
	}()

	_, err = store.pool.Exec(
		ctx,
		`
			INSERT INTO greybeard.states (
				check_name,
				state,
				output,
				changed_at,
				acknowledged_at
			)
			VALUES ($1, $2, $3, $4, now())
		`,
		name,
		int16(stateCritical),
		"CRITICAL - test",
		time.Now(),
	)

	if err != nil {
		t.Fatalf("unable to create acknowledged state: %v", err)
	}

	now := time.Now()

	result := Result{
		Name:       name,
		State:      stateOK,
		Output:     "OK - recovered\n",
		StartedAt:  now,
		FinishedAt: now,
	}

	if err := store.Record(ctx, result, true); err != nil {
		t.Fatalf("unable to record transition: %v", err)
	}

	var acknowledged bool

	err = store.pool.QueryRow(
		ctx,
		`
			SELECT acknowledged_at IS NOT NULL
			FROM greybeard.states
			WHERE check_name = $1
		`,
		name,
	).Scan(&acknowledged)

	if err != nil {
		t.Fatalf("unable to read acknowledgement: %v", err)
	}

	if acknowledged {
		t.Fatal("expected acknowledgement to be cleared after state transition")
	}
}

func TestPostgresResultStoreRecordWithoutStateUpdatePreservesAcknowledgement(t *testing.T) {
	databaseURL := os.Getenv("GREYBEARD_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GREYBEARD_DATABASE_URL is not set")
	}

	ctx := context.Background()

	store, err := NewPostgresResultStore(ctx, databaseURL)
	if err != nil {
		t.Fatalf("unable to create result store: %v", err)
	}

	name := "test-acknowledgement-preserved"

	defer func() {
		_, _ = store.pool.Exec(
			ctx,
			`
				DELETE FROM greybeard.metrics
				WHERE result_id IN (
					SELECT id
					FROM greybeard.results
					WHERE check_name = $1
				)
			`,
			name,
		)

		_, _ = store.pool.Exec(
			ctx,
			`DELETE FROM greybeard.results WHERE check_name = $1`,
			name,
		)

		_, _ = store.pool.Exec(
			ctx,
			`DELETE FROM greybeard.states WHERE check_name = $1`,
			name,
		)
	}()

	now := time.Now()

	_, err = store.pool.Exec(
		ctx,
		`
			INSERT INTO greybeard.states (
				check_name,
				state,
				output,
				changed_at,
				acknowledged_at
			)
			VALUES ($1, $2, $3, $4, now())
		`,
		name,
		int16(stateCritical),
		"CRITICAL - test",
		now,
	)

	if err != nil {
		t.Fatalf("unable to create acknowledged state: %v", err)
	}

	result := Result{
		Name:       name,
		State:      stateCritical,
		Output:     "CRITICAL - still broken\n",
		StartedAt:  now,
		FinishedAt: now,
	}

	if err := store.Record(ctx, result, false); err != nil {
		t.Fatalf("unable to record result: %v", err)
	}

	acknowledged, err := store.IsCriticalAcknowledged(ctx, name)
	if err != nil {
		t.Fatalf("unable to read acknowledgement: %v", err)
	}

	if !acknowledged {
		t.Fatal("expected acknowledgement to survive result without state update")
	}
}
