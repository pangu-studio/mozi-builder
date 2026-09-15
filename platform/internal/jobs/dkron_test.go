package jobs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pangu-studio/mozi-builder/mozi"
)

func TestDkronSyncPayload(t *testing.T) {
	var payload dkronJob
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(404)
			return
		}
		path = r.Method + " " + r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&payload)
		w.WriteHeader(200)
	}))
	defer srv.Close()
	c := DkronClient{BaseURL: srv.URL}
	job := &mozi.JobIR{
		Module: "content", Name: "DeckDigestJob", Schedule: "0 3 * * *",
		Executor: mozi.JobExecutorIR{Kind: "http", Method: "POST", Path: "/jobs/x"},
	}
	if err := c.SyncJob(context.Background(), job, "http://platform/api/v2/dkron/fire", "secret"); err != nil {
		t.Fatal(err)
	}
	if path != "POST /v1/jobs" {
		t.Fatalf("call: %s", path)
	}
	if payload.Retries != 0 {
		t.Fatal("Dkron-level retries must stay zero; business retries live in the platform")
	}
	if payload.Disabled || payload.Schedule != "0 3 * * *" || payload.ExecutorConfig["method"] != "POST" {
		t.Fatalf("payload: %+v", payload)
	}
	if !strings.Contains(payload.ExecutorConfig["url"], "fire_key=secret") {
		t.Fatalf("fire key missing from url: %+v", payload.ExecutorConfig)
	}
	// Disabled JobIR syncs as a disabled Dkron job (never run-triggered).
	off := false
	job.Enabled = &off
	if err := c.SyncJob(context.Background(), job, "http://platform/api/v2/dkron/fire", "secret"); err != nil {
		t.Fatal(err)
	}
	if !payload.Disabled {
		t.Fatal("disabled JobIR must sync as disabled Dkron job")
	}
}

func TestDkronDeleteToleratesMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()
	c := DkronClient{BaseURL: srv.URL}
	if err := c.DeleteJob(context.Background(), "content", "GoneJob"); err != nil {
		t.Fatalf("404 must be tolerated: %v", err)
	}
}

func TestDispatchProtocol(t *testing.T) {
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		w.WriteHeader(200)
	}))
	defer srv.Close()
	db := isolatedPlatformDB(t)
	project := createProject(t, db)
	s := Store{DB: db}
	ctx := context.Background()
	e, err := s.Trigger(ctx, project, "content", "DeckDigestJob", TriggerScheduled)
	if err != nil {
		t.Fatal(err)
	}
	d := Dispatcher{Store: s, GatewayURL: srv.URL}
	job := &mozi.JobIR{
		Module: "content", Name: "DeckDigestJob", TimeoutSeconds: 5,
		Executor: mozi.JobExecutorIR{Kind: "http", Method: "POST", Path: "/jobs/deck-digest"},
	}
	if err = d.Dispatch(ctx, e, job); err != nil {
		t.Fatal(err)
	}
	if headers.Get(HeaderExecutionID) != e.ExecutionID || headers.Get(HeaderAttempt) != "1" ||
		headers.Get(HeaderJob) != "content/DeckDigestJob" || headers.Get(HeaderTrigger) != TriggerScheduled {
		t.Fatalf("protocol headers: %v", headers)
	}
	got, _ := s.Get(ctx, e.ExecutionID)
	if got.State != Succeeded {
		t.Fatalf("state: %s", got.State)
	}
}

func TestDispatchFailureAndSlowExecutor(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer slow.Close()
	db := isolatedPlatformDB(t)
	project := createProject(t, db)
	s := Store{DB: db}
	ctx := context.Background()
	e, _ := s.Trigger(ctx, project, "content", "SlowJob", TriggerScheduled)
	// Timeout marks the attempt failed; the business work itself is untouched.
	d := Dispatcher{Store: s, GatewayURL: slow.URL, Client: &http.Client{Timeout: 50 * time.Millisecond}}
	job := &mozi.JobIR{
		Module: "content", Name: "SlowJob", TimeoutSeconds: 5,
		Executor: mozi.JobExecutorIR{Kind: "http", Method: "POST", Path: "/slow"},
	}
	if err := d.Dispatch(ctx, e, job); err == nil {
		t.Fatal("expected dispatch timeout error")
	}
	got, _ := s.Get(ctx, e.ExecutionID)
	errText := strings.ToLower(got.Error)
	if got.State != Failed || (!strings.Contains(errText, "timeout") && !strings.Contains(errText, "deadline")) {
		t.Fatalf("state: %s %q", got.State, got.Error)
	}
}
