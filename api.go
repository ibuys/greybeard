package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultLogLimit = 50
	maxLogLimit     = 200
	apiQueryTimeout = 5 * time.Second
)

var errIncidentChanged = errors.New("no active CRITICAL incident with that changed_at")

type checkStatus struct {
	Name         string          `json:"name"`
	Target       string          `json:"target"`
	Confirmed    *confirmedState `json:"confirmed"`
	LatestResult *resultSummary  `json:"latest_result"`
}

type confirmedState struct {
	State          string     `json:"state"`
	Output         string     `json:"output"`
	ChangedAt      time.Time  `json:"changed_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at"`
}

type resultSummary struct {
	State      string    `json:"state"`
	FinishedAt time.Time `json:"finished_at"`
}

type checkLog struct {
	ID     int64  `json:"id"`
	Target string `json:"target"`
	resultSummary
	Output        string    `json:"output"`
	StartedAt     time.Time `json:"started_at"`
	DurationNS    int64     `json:"duration_ns"`
	PerfData      string    `json:"perf_data"`
	PerfDataError string    `json:"perf_data_error"`
}

type logCursor struct {
	finishedAt *time.Time
	id         int64
}

type apiStore interface {
	CheckStatuses(context.Context, []string) ([]checkStatus, error)
	CheckLogs(context.Context, string, logCursor, int) ([]checkLog, error)
	AcknowledgeCritical(context.Context, string, time.Time) (time.Time, error)
}

type checkAPI struct {
	store  apiStore
	checks map[string]Check
	names  []string
}

func newAPI(store apiStore, checks []Check, token string) http.Handler {
	api := &checkAPI{store: store, checks: make(map[string]Check, len(checks))}
	for _, check := range checks {
		api.checks[check.Name] = check
		api.names = append(api.names, check.Name)
	}
	sort.Strings(api.names)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/checks", api.listChecks)
	mux.HandleFunc("GET /v1/checks/{name}", api.getCheck)
	mux.HandleFunc("GET /v1/checks/{name}/logs", api.getLogs)
	mux.HandleFunc("PUT /v1/checks/{name}/acknowledgement", api.acknowledge)

	expected := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		scheme, suppliedToken, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		supplied := sha256.Sum256([]byte(suppliedToken))
		if !ok || !strings.EqualFold(scheme, "Bearer") || subtle.ConstantTimeCompare(expected[:], supplied[:]) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeAPIError(w, http.StatusUnauthorized, "valid bearer token required")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), apiQueryTimeout)
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}

func startAPI(ctx context.Context, addr, token string, store apiStore, checks []Check) (*http.Server, <-chan error, error) {
	if token == "" || strings.ContainsAny(token, " \t\r\n") {
		return nil, nil, errors.New("GREYBEARD_API_TOKEN must be set to a token without whitespace")
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}

	server := &http.Server{
		Handler:           newAPI(store, checks, token),
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    8 << 10,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	failures := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
			failures <- err
		}
	}()
	fmt.Fprintf(os.Stderr, "Greybeard API listening on %s\n", listener.Addr())
	return server, failures, nil
}

func (api *checkAPI) statuses(ctx context.Context, names []string) ([]checkStatus, error) {
	statuses, err := api.store.CheckStatuses(ctx, names)
	if err != nil {
		return nil, err
	}
	for i := range statuses {
		statuses[i].Target = api.checks[statuses[i].Name].Target
	}
	return statuses, nil
}

func (api *checkAPI) listChecks(w http.ResponseWriter, r *http.Request) {
	statuses, err := api.statuses(r.Context(), api.names)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "could not load check status")
		return
	}
	writeAPIJSON(w, http.StatusOK, struct {
		Checks []checkStatus `json:"checks"`
	}{Checks: statuses})
}

func (api *checkAPI) getCheck(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !api.hasCheck(w, name) {
		return
	}
	statuses, err := api.statuses(r.Context(), []string{name})
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "could not load check status")
		return
	}
	writeAPIJSON(w, http.StatusOK, statuses[0])
}

func (api *checkAPI) hasCheck(w http.ResponseWriter, name string) bool {
	_, exists := api.checks[name]
	if !exists {
		writeAPIError(w, http.StatusNotFound, "check not found")
	}
	return exists
}

func (api *checkAPI) getLogs(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !api.hasCheck(w, name) {
		return
	}

	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid query parameters")
		return
	}
	limit := defaultLogLimit
	if query.Has("limit") {
		limit, err = strconv.Atoi(query.Get("limit"))
		if err != nil || limit < 1 || limit > maxLogLimit {
			writeAPIError(w, http.StatusBadRequest, "limit must be an integer from 1 to 200")
			return
		}
	}

	var cursor logCursor
	if query.Has("before") {
		value, id, _ := strings.Cut(query.Get("before"), "~")
		finishedAt, timeErr := time.Parse(time.RFC3339Nano, value)
		resultID, idErr := strconv.ParseInt(id, 10, 64)
		if timeErr != nil || idErr != nil || resultID <= 0 {
			writeAPIError(w, http.StatusBadRequest, "invalid before cursor")
			return
		}
		cursor = logCursor{finishedAt: &finishedAt, id: resultID}
	}

	logs, err := api.store.CheckLogs(r.Context(), name, cursor, limit+1)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "could not load check logs")
		return
	}

	var nextBefore string
	if len(logs) > limit {
		logs = logs[:limit]
		last := logs[limit-1]
		nextBefore = last.FinishedAt.Format(time.RFC3339Nano) + "~" + strconv.FormatInt(last.ID, 10)
	}
	writeAPIJSON(w, http.StatusOK, struct {
		Logs       []checkLog `json:"logs"`
		NextBefore string     `json:"next_before,omitempty"`
	}{Logs: logs, NextBefore: nextBefore})
}

func (api *checkAPI) acknowledge(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !api.hasCheck(w, name) {
		return
	}

	var body struct {
		ChangedAt time.Time `json:"changed_at"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || body.ChangedAt.IsZero() ||
		body.ChangedAt.Nanosecond()%1000 != 0 || decoder.Decode(new(json.RawMessage)) != io.EOF {
		writeAPIError(w, http.StatusBadRequest, "expected a JSON object with a changed_at timestamp")
		return
	}

	at, err := api.store.AcknowledgeCritical(r.Context(), name, body.ChangedAt)
	switch {
	case errors.Is(err, errIncidentChanged):
		writeAPIError(w, http.StatusConflict, errIncidentChanged.Error())
	case err != nil:
		writeAPIError(w, http.StatusInternalServerError, "could not save acknowledgement")
	default:
		writeAPIJSON(w, http.StatusOK, struct {
			Name           string    `json:"name"`
			ChangedAt      time.Time `json:"changed_at"`
			AcknowledgedAt time.Time `json:"acknowledged_at"`
		}{Name: name, ChangedAt: body.ChangedAt.UTC(), AcknowledgedAt: at})
	}
}

func writeAPIError(w http.ResponseWriter, status int, message string) {
	writeAPIJSON(w, status, map[string]string{"error": message})
}

func writeAPIJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
