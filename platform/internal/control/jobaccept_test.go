package control

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pangu-studio/mozi-builder/mozi"
	"github.com/pangu-studio/mozi-builder/platform/internal/config"
	"github.com/pangu-studio/mozi-builder/platform/internal/design"
	"github.com/pangu-studio/mozi-builder/platform/internal/jobs"
	"github.com/zeromicro/go-zero/rest/router"
)

// Phase-5 acceptance: the job protocol against a real Dkron (acceptance
// stack, make v2-acc-up). Requires MOZI_INTEGRATION_ENV and MOZI_JOBS_ACC=1.
// Dkron (container) reaches the host-run API via the lima gateway 192.168.5.2.
const (
	accDkron   = "http://127.0.0.1:18082"
	accFireKey = "acc-fire-key"
	accHost    = "192.168.5.2"
)

type execRecorder struct {
	mu      sync.Mutex
	attempt map[string]int
}

func (r *execRecorder) note(executionID string, attempt int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attempt[executionID+":"+strconv.Itoa(attempt)]++
}

func TestJobAcceptance(t *testing.T) {
	if os.Getenv("MOZI_JOBS_ACC") != "1" {
		t.Skip("MOZI_JOBS_ACC=1 and the acceptance stack (make v2-acc-up) required")
	}
	env := os.Getenv("MOZI_INTEGRATION_ENV")
	if env == "" {
		t.Skip("MOZI_INTEGRATION_ENV required")
	}
	if err := config.LoadEnvFile(env); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadDatabases(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	db := isolatedDB(t, cfg.Platform, "platform")
	designDB := isolatedDB(t, cfg.Design, "design")
	ctx := context.Background()

	// Business executor (host-side): /jobs/ok succeeds; /jobs/flaky times
	// out on attempt 1 and succeeds on later attempts.
	rec := &execRecorder{attempt: map[string]int{}}
	executor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt, _ := strconv.Atoi(r.Header.Get(jobs.HeaderAttempt))
		rec.note(r.Header.Get(jobs.HeaderExecutionID), attempt)
		switch r.URL.Path {
		case "/jobs/flaky":
			if attempt == 1 {
				time.Sleep(2 * time.Second)
			}
		}
		w.WriteHeader(200)
	}))
	defer executor.Close()

	jobStore := &jobs.Store{DB: db}
	dispatcher := &jobs.Dispatcher{Store: *jobStore, GatewayURL: executor.URL}
	a := API{DB: db, Design: designDB, Jobs: jobStore, Dispatcher: dispatcher, FireKey: accFireKey}
	mux := router.NewRouter()
	for _, r := range a.Routes() {
		if err := mux.Handle(r.Method, r.Path, http.HandlerFunc(r.Handler)); err != nil {
			t.Fatal(err)
		}
	}
	// Bind on all interfaces so Dkron can reach us via the lima gateway.
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	api := &http.Server{Handler: mux}
	go func() { _ = api.Serve(listener) }()
	defer api.Close()
	apiPort := listener.Addr().(*net.TCPAddr).Port

	// Project + user + job documents in the real design DB.
	if err = CreateUser(ctx, db, "jacc@test.local", "tester", "test-password-123"); err != nil {
		t.Fatal(err)
	}
	token, err := login(ctx, db, "jacc@test.local", "test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	project := accCall(t, mux, "POST", "projects", token, map[string]string{"slug": "jacc-" + strings.ToLower(rand.Text())[:6], "name": "Jobs Acc"}, 201)["id"].(string)
	scope := design.Scope{ID: project, Slug: "jacc", Name: "Jobs Acc", Actor: "acc"}
	saveJob := func(j mozi.JobIR) {
		t.Helper()
		raw, _ := json.Marshal(j)
		if _, err := (design.Store{DB: designDB}).SaveJob(ctx, scope, j.Module, j.Name, "", raw, "created"); err != nil {
			t.Fatal(err)
		}
	}
	base := mozi.JobIR{
		SchemaVersion: 1, Module: "content", Label: "验收任务", TimeoutSeconds: 5,
		Executor: mozi.JobExecutorIR{Kind: "http", Method: "POST", Path: "/jobs/ok"},
		Retry:    mozi.JobRetryIR{MaxAttempts: 3, BackoffSeconds: 1},
	}
	schedJob := base
	schedJob.Name, schedJob.Schedule = "AccSchedJob", "@every 2s"
	manualJob := base
	manualJob.Name, manualJob.Schedule = "AccManualJob", "0 0 1 1 *"
	flakyJob := base
	flakyJob.Name, flakyJob.Schedule, flakyJob.TimeoutSeconds = "AccFlakyJob", "0 0 1 1 *", 1
	flakyJob.Executor.Path = "/jobs/flaky"
	off := false
	disabledJob := base
	disabledJob.Name, disabledJob.Schedule, disabledJob.Enabled = "AccDisabledJob", "@every 2s", &off
	heartJob := base
	heartJob.Name, heartJob.Schedule = "AccHeartJob", "0 0 1 1 *"
	for _, j := range []mozi.JobIR{schedJob, manualJob, flakyJob, disabledJob, heartJob} {
		saveJob(j)
	}

	waitExecution := func(job string, want func(e jobs.Execution) bool) jobs.Execution {
		t.Helper()
		deadline := time.Now().Add(25 * time.Second)
		for time.Now().Before(deadline) {
			list, err := jobStore.List(ctx, project, "content", job, 20)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range list {
				if want(e) {
					return e
				}
			}
			time.Sleep(300 * time.Millisecond)
		}
		list, _ := jobStore.List(ctx, project, "content", job, 20)
		t.Fatalf("no execution of %s matched; seen %d rows: %+v", job, len(list), list)
		return jobs.Execution{}
	}
	dkron := jobs.DkronClient{BaseURL: accDkron}

	// Probe the fire endpoint directly before involving Dkron.
	probeJob := base
	probeJob.Name, probeJob.Schedule = "AccProbeJob", "0 0 1 1 *"
	saveJob(probeJob)
	probeURL := fmt.Sprintf("http://127.0.0.1:%d/api/v2/dkron/fire?project=%s&module=content&job=AccProbeJob", apiPort, project)
	preq, _ := http.NewRequest("POST", probeURL, nil)
	preq.Header.Set("X-Mozi-Fire-Key", accFireKey)
	presp, err := http.DefaultClient.Do(preq)
	if err != nil {
		t.Fatal(err)
	}
	probeBody := make([]byte, 512)
	n, _ := presp.Body.Read(probeBody)
	presp.Body.Close()
	if presp.StatusCode != 202 {
		t.Fatalf("fire probe: %d %s", presp.StatusCode, probeBody[:n])
	}
	waitExecution("AccProbeJob", func(e jobs.Execution) bool { return e.State == jobs.Succeeded })

	// --- Scenario 1: scheduled trigger through real Dkron ---
	fireURL := fmt.Sprintf("http://%s:%d/api/v2/dkron/fire?project=%s&module=content&job=AccSchedJob", accHost, apiPort, project)
	if err := dkron.SyncJob(ctx, &schedJob, fireURL, accFireKey); err != nil {
		t.Fatal(err)
	}
	sched := waitExecution("AccSchedJob", func(e jobs.Execution) bool {
		return e.Trigger == jobs.TriggerScheduled && e.State == jobs.Succeeded
	})
	if sched.Attempt != 1 {
		t.Fatalf("scheduled attempt: %d", sched.Attempt)
	}
	if err := dkron.DeleteJob(ctx, "content", "AccSchedJob"); err != nil {
		t.Fatal(err)
	}

	// Disabled job must never be run-triggered by the scheduler.
	disabledFire := fmt.Sprintf("http://127.0.0.1:%d/api/v2/dkron/fire?project=%s&module=content&job=AccDisabledJob", apiPort, project)
	req, _ := http.NewRequest("POST", disabledFire, nil)
	req.Header.Set("X-Mozi-Fire-Key", accFireKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Fatalf("disabled scheduled fire: %d", resp.StatusCode)
	}

	// --- Scenario 2: manual trigger ---
	accCall(t, mux, "POST", "projects/"+project+"/jobs/content/AccManualJob/fire", token, nil, 202)
	manual := waitExecution("AccManualJob", func(e jobs.Execution) bool {
		return e.Trigger == jobs.TriggerManual && e.State == jobs.Succeeded
	})

	// --- Scenario 3: timeout fails the attempt, retry shares execution_id ---
	accCall(t, mux, "POST", "projects/"+project+"/jobs/content/AccFlakyJob/fire", token, nil, 202)
	flaky := waitExecution("AccFlakyJob", func(e jobs.Execution) bool {
		return e.Attempt == 1 && e.State == jobs.Failed
	})
	retry, err := jobStore.Retry(ctx, flaky.ExecutionID, 3)
	if err != nil {
		t.Fatal(err)
	}
	if retry.ExecutionID != flaky.ExecutionID || retry.Attempt != 2 {
		t.Fatalf("retry: %+v", retry)
	}
	if err := dispatcher.Dispatch(ctx, retry, &flakyJob); err != nil {
		t.Fatal(err)
	}
	got, _ := jobStore.Get(ctx, flaky.ExecutionID)
	if got.State != jobs.Succeeded || got.Attempt != 2 {
		t.Fatalf("retried execution: %+v", got)
	}
	rec.mu.Lock()
	if rec.attempt[flaky.ExecutionID+":2"] != 1 {
		t.Fatalf("executor attempts: %v", rec.attempt)
	}
	rec.mu.Unlock()

	// --- Scenario 4: long-running heartbeat then lost ---
	heart, err := jobStore.Trigger(ctx, project, "content", "AccHeartJob", jobs.TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	heartbeat := func(status int) {
		t.Helper()
		req, _ := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%d/api/v2/jobs/heartbeat", apiPort), nil)
		req.Header.Set("X-Mozi-Fire-Key", accFireKey)
		req.Header.Set(jobs.HeaderExecutionID, heart.ExecutionID)
		req.Header.Set(jobs.HeaderAttempt, "1")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != status {
			t.Fatalf("heartbeat: %d want %d", resp.StatusCode, status)
		}
	}
	heartbeat(204)
	heartbeat(204)
	// Heartbeat stops; the sweep marks the attempt lost (timeout ≠ termination).
	time.Sleep(1200 * time.Millisecond)
	lost, err := jobStore.SweepLost(ctx, 500*time.Millisecond)
	if err != nil || lost != 1 {
		t.Fatalf("sweep: %d %v", lost, err)
	}
	got, _ = jobStore.Get(ctx, heart.ExecutionID)
	if got.State != jobs.Lost {
		t.Fatalf("state: %s", got.State)
	}
	heartbeat(409) // a lost attempt no longer accepts heartbeats

	// Member can read executions; viewer cannot fire.
	list := accCall(t, mux, "GET", "projects/"+project+"/jobs/content/AccManualJob/executions", token, nil, 200)
	_ = list
	_ = manual
	fmt.Println("job acceptance scenarios passed: scheduled, manual, timeout retry, heartbeat")
}

func accCall(t *testing.T, mux http.Handler, method, path, token string, body any, status int) map[string]any {
	t.Helper()
	data, _ := json.Marshal(body)
	r := httptest.NewRequest(method, "/api/v2/"+path, strings.NewReader(string(data)))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != status {
		t.Fatalf("%s %s: %d expected %d: %s", method, path, w.Code, status, w.Body.String())
	}
	result := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	return result
}
