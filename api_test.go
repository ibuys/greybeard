package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

const testAPIToken = "test-token"

func requestAPI(handler http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestAPIRejectsInvalidRequests(t *testing.T) {
	handler := newAPI(nil, []Check{{Name: "example"}}, testAPIToken)
	tests := []struct {
		name, method, path, token, body string
		status                          int
	}{
		{"missing token", "GET", "/v1/checks", "", "", 401},
		{"wrong token", "GET", "/v1/checks", "wrong", "", 401},
		{"missing check", "GET", "/v1/checks/missing", testAPIToken, "", 404},
		{"wrong method", "POST", "/v1/checks", testAPIToken, "", 405},
		{"bad limit", "GET", "/v1/checks/example/logs?limit=201", testAPIToken, "", 400},
		{"bad cursor", "GET", "/v1/checks/example/logs?before=invalid", testAPIToken, "", 400},
		{"missing timestamp", "PUT", "/v1/checks/example/acknowledgement", testAPIToken, "{}", 400},
		{"trailing data", "PUT", "/v1/checks/example/acknowledgement", testAPIToken, `{"changed_at":"2026-01-01T00:00:00Z"}{}`, 400},
		{"oversized body", "PUT", "/v1/checks/example/acknowledgement", testAPIToken, strings.Repeat(" ", 4097), 400},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := requestAPI(handler, test.method, test.path, test.token, test.body)
			if response.Code != test.status {
				t.Fatalf("got HTTP %d, want %d: %s", response.Code, test.status, response.Body)
			}
			if test.status == 401 && response.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("missing bearer challenge")
			}
		})
	}
}

func TestAPIWithPostgres(t *testing.T) {
	databaseURL := os.Getenv("GREYBEARD_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GREYBEARD_TEST_DATABASE_URL is not set")
	}

	ctx := t.Context()
	store, err := NewPostgresResultStore(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}

	name := "test-api/" + time.Now().UTC().Format(time.RFC3339Nano)
	defer store.Close()
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, query := range []string{
			"DELETE FROM greybeard.results WHERE check_name = $1",
			"DELETE FROM greybeard.states WHERE check_name = $1",
		} {
			if _, err := store.pool.Exec(cleanup, query, name); err != nil {
				t.Error(err)
			}
		}
	}()
	handler := newAPI(store, []Check{{Name: name, Target: "target"}, {Name: "pending"}}, testAPIToken)
	checkPath := "/v1/checks/" + url.PathEscape(name)
	call := func(method, path, body string, want int, destination any) {
		t.Helper()
		response := requestAPI(handler, method, path, testAPIToken, body)
		if response.Code != want {
			t.Fatalf("%s %s returned HTTP %d, want %d: %s", method, path, response.Code, want, response.Body)
		}
		if destination != nil {
			if err := json.Unmarshal(response.Body.Bytes(), destination); err != nil {
				t.Fatal(err)
			}
		}
	}

	var listing struct{ Checks []checkStatus }
	call(http.MethodGet, "/v1/checks", "", 200, &listing)
	if len(listing.Checks) != 2 || listing.Checks[0].Name != "pending" || listing.Checks[1].Name != name ||
		listing.Checks[1].Target != "target" || listing.Checks[1].Confirmed != nil || listing.Checks[1].LatestResult != nil {
		t.Fatalf("unexpected list for configured checks: %+v", listing.Checks)
	}

	var logs struct {
		Logs       []checkLog
		NextBefore string `json:"next_before"`
	}
	call(http.MethodGet, checkPath+"/logs", "", 200, &logs)
	if logs.Logs == nil || len(logs.Logs) != 0 {
		t.Fatalf("expected empty log array, got %+v", logs.Logs)
	}

	first := time.Date(2026, 1, 1, 1, 2, 3, 456789000, time.UTC)
	record := func(state State, at time.Time, output string, transition bool) {
		t.Helper()
		if err := store.Record(ctx, Result{
			Name: name, State: state, Target: "target", Output: output, StartedAt: at.Add(-time.Second),
			FinishedAt: at, Duration: time.Second, PerfData: "value=42",
		}, transition); err != nil {
			t.Fatal(err)
		}
	}
	record(stateCritical, first, "CRITICAL - confirmed", true)
	record(stateWarning, first, "WARNING - candidate", false)

	var status checkStatus
	call(http.MethodGet, checkPath, "", 200, &status)
	if status.Confirmed == nil || status.Confirmed.State != "CRITICAL" || !status.Confirmed.ChangedAt.Equal(first) ||
		status.LatestResult == nil || status.LatestResult.State != "WARNING" || status.Confirmed.AcknowledgedAt != nil {
		t.Fatalf("unexpected confirmed or observed state: %+v / %+v", status.Confirmed, status.LatestResult)
	}

	call(http.MethodGet, checkPath+"/logs?limit=1", "", 200, &logs)
	if len(logs.Logs) != 1 || logs.Logs[0].Output != "WARNING - candidate" || logs.NextBefore == "" {
		t.Fatalf("unexpected first page: %+v", logs)
	}
	before := logs.NextBefore
	logs.NextBefore = ""
	call(http.MethodGet, checkPath+"/logs?limit=1&before="+url.QueryEscape(before), "", 200, &logs)
	if len(logs.Logs) != 1 || logs.Logs[0].Output != "CRITICAL - confirmed" || logs.Logs[0].PerfData != "value=42" || logs.NextBefore != "" {
		t.Fatalf("unexpected second page: %+v", logs)
	}

	ackPath := checkPath + "/acknowledgement"
	body := `{"changed_at":"` + status.Confirmed.ChangedAt.Format(time.RFC3339Nano) + `"}`
	var firstAck, retry struct {
		At time.Time `json:"acknowledged_at"`
	}
	call(http.MethodPut, ackPath, body, 200, &firstAck)
	call(http.MethodPut, ackPath, body, 200, &retry)
	if firstAck.At.IsZero() || !firstAck.At.Equal(retry.At) {
		t.Fatal("acknowledgement must return and retain its original timestamp")
	}
	if acknowledged, err := store.IsCriticalAcknowledged(ctx, name); err != nil || !acknowledged {
		t.Fatalf("reminder suppression sees acknowledged=%t, err=%v", acknowledged, err)
	}
	call(http.MethodGet, checkPath, "", 200, &status)
	if status.Confirmed.AcknowledgedAt == nil || !status.Confirmed.AcknowledgedAt.Equal(firstAck.At) {
		t.Fatalf("status should include acknowledgement: %+v", status.Confirmed)
	}

	record(stateOK, first.Add(time.Second), "OK - recovered", true)
	call(http.MethodPut, ackPath, body, 409, nil)
	record(stateCritical, first.Add(2*time.Second), "CRITICAL - new incident", true)
	call(http.MethodPut, ackPath, body, 409, nil)
	if acknowledged, err := store.IsCriticalAcknowledged(ctx, name); err != nil || acknowledged {
		t.Fatalf("new incident sees acknowledged=%t, err=%v", acknowledged, err)
	}
}
