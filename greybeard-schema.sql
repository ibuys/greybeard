--
-- PostgreSQL database dump
--

\restrict Yxe2sPqUh5xI6swbElTVdofgP6Srj9wAfsJyPhGLMPqWQF8ylbqp9LreenevQVR

-- Dumped from database version 18.4 (Debian 18.4-1.pgdg13+1)
-- Dumped by pg_dump version 18.6 (Homebrew)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: greybeard; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA greybeard;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: metrics; Type: TABLE; Schema: greybeard; Owner: -
--

CREATE TABLE greybeard.metrics (
    result_id bigint NOT NULL,
    ordinal integer NOT NULL,
    name text NOT NULL,
    value double precision,
    unit text NOT NULL,
    warning text NOT NULL,
    critical text NOT NULL,
    minimum double precision,
    maximum double precision
);


--
-- Name: results; Type: TABLE; Schema: greybeard; Owner: -
--

CREATE TABLE greybeard.results (
    id bigint NOT NULL,
    check_name text NOT NULL,
    state smallint NOT NULL,
    output text NOT NULL,
    perf_data text NOT NULL,
    perf_data_error text NOT NULL,
    started_at timestamp with time zone NOT NULL,
    finished_at timestamp with time zone NOT NULL,
    duration_ns bigint NOT NULL,
    CONSTRAINT results_duration_ns_check CHECK ((duration_ns >= 0)),
    CONSTRAINT results_state_check CHECK (((state >= 0) AND (state <= 3)))
);


--
-- Name: results_id_seq; Type: SEQUENCE; Schema: greybeard; Owner: -
--

ALTER TABLE greybeard.results ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME greybeard.results_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: states; Type: TABLE; Schema: greybeard; Owner: -
--

CREATE TABLE greybeard.states (
    check_name text NOT NULL,
    state smallint NOT NULL,
    output text NOT NULL,
    changed_at timestamp with time zone NOT NULL,
    acknowledged_at timestamp with time zone,
    CONSTRAINT states_state_check CHECK (((state >= 0) AND (state <= 3)))
);


--
-- Name: metrics metrics_pkey; Type: CONSTRAINT; Schema: greybeard; Owner: -
--

ALTER TABLE ONLY greybeard.metrics
    ADD CONSTRAINT metrics_pkey PRIMARY KEY (result_id, ordinal);


--
-- Name: results results_pkey; Type: CONSTRAINT; Schema: greybeard; Owner: -
--

ALTER TABLE ONLY greybeard.results
    ADD CONSTRAINT results_pkey PRIMARY KEY (id);


--
-- Name: states states_pkey; Type: CONSTRAINT; Schema: greybeard; Owner: -
--

ALTER TABLE ONLY greybeard.states
    ADD CONSTRAINT states_pkey PRIMARY KEY (check_name);


--
-- Name: results_check_finished_idx; Type: INDEX; Schema: greybeard; Owner: -
--

CREATE INDEX results_check_finished_idx ON greybeard.results USING btree (check_name, finished_at DESC);


--
-- Name: metrics metrics_result_id_fkey; Type: FK CONSTRAINT; Schema: greybeard; Owner: -
--

ALTER TABLE ONLY greybeard.metrics
    ADD CONSTRAINT metrics_result_id_fkey FOREIGN KEY (result_id) REFERENCES greybeard.results(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict Yxe2sPqUh5xI6swbElTVdofgP6Srj9wAfsJyPhGLMPqWQF8ylbqp9LreenevQVR

