CREATE SCHEMA IF NOT EXISTS greybeard;

CREATE TABLE greybeard.results (
	id				bigint 		GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	check_name		text		NOT NULL,
	state			smallint	NOT NULL CHECK (state BETWEEN 0 AND 3),
	output			text		NOT NULL,
	perf_data		text		NOT NULL,
	perf_data_error text		NOT NULL,
	started_at		timestamptz NOT NULL,
	finished_at		timestamptz NOT NULL,
	duration_ns		bigint		NOT NULL CHECK (duration_ns >= 0)
);

CREATE INDEX results_check_finished_idx
	ON greybeard.results (check_name, finished_at DESC);

CREATE TABLE greybeard.metrics (
	result_id		bigint		NOT NULL
		REFERENCES greybeard.results(id)
		ON DELETE CASCADE,
	
	ordinal 		integer		NOT NULL,
	name			text		NOT NULL,
	value			double precision,
	unit			text		NOT NULL,
	warning			text		NOT NULL,
	critical		text		NOT NULL,
	minimum			double precision,
	maximum			double precision,
	
	PRIMARY KEY (result_id, ordinal)

);


CREATE TABLE greybeard.states (
	check_name			text			PRIMARY KEY,
	state				smallint		NOT NULL CHECK (state BETWEEN 0 and 3),
	output				text			NOT NULL,
	changed_at			timestamptz		NOT NULL
);