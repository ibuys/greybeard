package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresResultStore struct {
	pool *pgxpool.Pool
}

func NewPostgresResultStore(
	ctx context.Context,
	connectionString string,
) (*PostgresResultStore, error) {

	pool, err := pgxpool.New(ctx, connectionString)
	if err != nil {
		return nil, fmt.Errorf("unable to create PostgreSQL connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to connect to PostgreSQL: %w", err)
	}

	return &PostgresResultStore{
		pool: pool,
	}, nil
}

func (store *PostgresResultStore) LoadStates(ctx context.Context, names []string) (map[string]State, error) {

	rows, err := store.pool.Query(
		ctx,
		`
				SELECT
					check_name,
					state
				FROM greybeard.states
				WHERE check_name = ANY($1)
			`,
		names,
	)

	if err != nil {
		return nil, fmt.Errorf("unable to load check states: %w", err)
	}

	defer rows.Close()

	states := make(map[string]State)

	for rows.Next() {
		var name string
		var stateValue int16

		err := rows.Scan(
			&name,
			&stateValue,
		)

		if err != nil {
			return nil, fmt.Errorf("unable to read check state: %w", err)
		}

		if stateValue < int16(stateOK) || stateValue > int16(stateUnknown) {
			return nil, fmt.Errorf(
				"invalid persisted state %d for check %q",
				stateValue,
				name,
			)
		}

		states[name] = State(stateValue)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("unable to load check states: %w", err)
	}

	return states, nil
}

func (store *PostgresResultStore) Record(
	ctx context.Context,
	result Result,
	updateState bool,
) error {

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf(
			"unable to begin result transaction: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var resultID int64

	if err := tx.QueryRow(
		ctx,
		`
			INSERT INTO greybeard.results (
				check_name,
				state,
				output,
				perf_data,
				perf_data_error,
				started_at,
				finished_at,
				duration_ns
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id
		`,
		result.Name,
		int16(result.State),
		result.Output,
		result.PerfData,
		result.PerfDataError,
		result.StartedAt,
		result.FinishedAt,
		result.Duration.Nanoseconds(),
	).Scan(&resultID); err != nil {
		return fmt.Errorf(
			"unable to record result for %q: %w",
			result.Name,
			err,
		)
	}

	for i, metric := range result.Metrics {

		if _, err := tx.Exec(
			ctx,
			`
				INSERT INTO greybeard.metrics (
					result_id,
					ordinal,
					name,
					value,
					unit,
					warning,
					critical,
					minimum,
					maximum
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`,
			resultID,
			i,
			metric.Name,
			metric.Value,
			metric.Unit,
			metric.Warning,
			metric.Critical,
			metric.Minimum,
			metric.Maximum,
		); err != nil {
			return fmt.Errorf(
				"unable to record metric %q for %q: %w",
				metric.Name,
				result.Name,
				err,
			)
		}
	}

	if updateState {
		if _, err = tx.Exec(
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
				changed_at= EXCLUDED.changed_at
		`,
			result.Name,
			int16(result.State),
			result.Output,
			result.FinishedAt,
		); err != nil {
			return fmt.Errorf(
				"unable to update state for %q: %w",
				result.Name,
				err,
			)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf(
			"unable to commit result for %q: %w",
			result.Name,
			err,
		)
	}

	return nil
}

func (store *PostgresResultStore) Close() {

}
