package main

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (store *PostgresResultStore) CheckStatuses(ctx context.Context, names []string) ([]checkStatus, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT checks.name, s.state, COALESCE(s.output, ''), s.changed_at, s.acknowledged_at,
		       latest.state, latest.finished_at
		FROM unnest($1::text[]) WITH ORDINALITY AS checks(name, position)
		LEFT JOIN greybeard.states AS s ON s.check_name = checks.name
		LEFT JOIN LATERAL (
			SELECT state, finished_at
			FROM greybeard.results
			WHERE check_name = checks.name
			ORDER BY finished_at DESC, id DESC
			LIMIT 1
		) AS latest ON true
		ORDER BY checks.position
	`, names)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	statuses := make([]checkStatus, 0, len(names))
	for rows.Next() {
		var status checkStatus
		var confirmedCode, latestCode *int16
		var output string
		var changedAt, acknowledgedAt, finishedAt *time.Time
		if err := rows.Scan(&status.Name, &confirmedCode, &output, &changedAt, &acknowledgedAt, &latestCode, &finishedAt); err != nil {
			return nil, err
		}
		if confirmedCode != nil {
			confirmed := State(*confirmedCode)
			status.Confirmed = &confirmedState{
				State:     confirmed.String(),
				Output:    output,
				ChangedAt: changedAt.UTC(),
			}
			if confirmed == stateCritical && acknowledgedAt != nil {
				at := acknowledgedAt.UTC()
				status.Confirmed.AcknowledgedAt = &at
			}
		}
		if latestCode != nil {
			status.LatestResult = &resultSummary{State: State(*latestCode).String(), FinishedAt: finishedAt.UTC()}
		}
		statuses = append(statuses, status)
	}
	return statuses, rows.Err()
}

func (store *PostgresResultStore) CheckLogs(ctx context.Context, name string, before logCursor, limit int) ([]checkLog, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT id, target, state, output, started_at, finished_at, duration_ns, perf_data, perf_data_error
		FROM greybeard.results
		WHERE check_name = $1 AND (finished_at, id) < (COALESCE($2::timestamptz, 'infinity'), $3)
		ORDER BY finished_at DESC, id DESC
		LIMIT $4
	`, name, before.finishedAt, before.id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := make([]checkLog, 0, limit)
	for rows.Next() {
		var entry checkLog
		var state int16
		if err := rows.Scan(&entry.ID, &entry.Target, &state, &entry.Output, &entry.StartedAt, &entry.FinishedAt,
			&entry.DurationNS, &entry.PerfData, &entry.PerfDataError); err != nil {
			return nil, err
		}
		entry.State = State(state).String()
		entry.StartedAt = entry.StartedAt.UTC()
		entry.FinishedAt = entry.FinishedAt.UTC()
		logs = append(logs, entry)
	}
	return logs, rows.Err()
}

func (store *PostgresResultStore) AcknowledgeCritical(ctx context.Context, name string, changedAt time.Time) (time.Time, error) {
	var acknowledgedAt time.Time
	err := store.pool.QueryRow(ctx, `
		UPDATE greybeard.states
		SET acknowledged_at = COALESCE(acknowledged_at, now())
		WHERE check_name = $1 AND state = $2 AND changed_at = $3
		RETURNING acknowledged_at
	`, name, int16(stateCritical), changedAt).Scan(&acknowledgedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, errIncidentChanged
	}
	return acknowledgedAt.UTC(), err
}
