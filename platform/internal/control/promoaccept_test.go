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
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/pangu-studio/mozi-builder/mozi"
	"github.com/pangu-studio/mozi-builder/platform/internal/config"
	"github.com/pangu-studio/mozi-builder/platform/internal/design"
	"github.com/pangu-studio/mozi-builder/platform/internal/jobs"
	"github.com/zeromicro/go-zero/rest/router"
)

// Phase-6 acceptance: promotion against the real Dkron in the acceptance
// stack. Requires MOZI_INTEGRATION_ENV and MOZI_PROMO_ACC=1.
func TestPromotionAcceptance(t *testing.T) {
	if os.Getenv("MOZI_PROMO_ACC") != "1" {
		t.Skip("MOZI_PROMO_ACC=1 and the acceptance stack (make v2-acc-up) required")
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

	executor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer executor.Close()
	jobStore := &jobs.Store{DB: db}
	dispatcher := &jobs.Dispatcher{Store: *jobStore, GatewayURL: executor.URL}
	dkron := &jobs.DkronClient{BaseURL: "http://127.0.0.1:18082"}
	// Reserve the API port before registering routes: handlers capture the
	// API value at registration, so FireURL must be set beforehand.
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	apiPort := listener.Addr().(*net.TCPAddr).Port
	a := API{DB: db, Design: designDB, Jobs: jobStore, Dispatcher: dispatcher, Dkron: dkron,
		FireURL: fmt.Sprintf("http://192.168.5.2:%d/api/v2/dkron/fire", apiPort), FireKey: accFireKey}
	mux := router.NewRouter()
	for _, r := range a.Routes() {
		if err = mux.Handle(r.Method, r.Path, http.HandlerFunc(r.Handler)); err != nil {
			t.Fatal(err)
		}
	}
	api := &http.Server{Handler: mux}
	go func() { _ = api.Serve(listener) }()
	defer api.Close()

	if err = CreateUser(ctx, db, "pacc@test.local", "tester", "test-password-123"); err != nil {
		t.Fatal(err)
	}
	token, err := login(ctx, db, "pacc@test.local", "test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	project := accCall(t, mux, "POST", "projects", token, map[string]string{"slug": "pacc-" + strings.ToLower(rand.Text())[:6], "name": "Promo Acc"}, 201)["id"].(string)
	dev := accCall(t, mux, "POST", "projects/"+project+"/environments", token, map[string]string{"slug": "dev", "name": "开发", "kind": "development"}, 201)["id"].(string)
	prod := accCall(t, mux, "POST", "projects/"+project+"/environments", token, map[string]string{"slug": "prod", "name": "生产", "kind": "production"}, 201)["id"].(string)

	suffix := strings.ToLower(rand.Text())[:6]
	jobName := "AccPromo" + suffix
	scope := design.Scope{ID: project, Slug: "pacc", Name: "Promo Acc", Actor: "acc"}
	jobVersion := ""
	saveJob := func(schedule string) mozi.JobIR {
		t.Helper()
		j := mozi.JobIR{
			SchemaVersion: 1, Module: "content", Name: jobName, Label: "晋级验收",
			Schedule: schedule, TimeoutSeconds: 5,
			Executor: mozi.JobExecutorIR{Kind: "http", Method: "POST", Path: "/jobs/ok"},
			Retry:    mozi.JobRetryIR{MaxAttempts: 1, BackoffSeconds: 60},
		}
		raw, _ := json.Marshal(j)
		action := "created"
		if jobVersion != "" {
			action = "updated"
		}
		saved, err := (design.Store{DB: designDB}).SaveJob(ctx, scope, j.Module, j.Name, jobVersion, raw, action)
		if err != nil {
			t.Fatal(err)
		}
		jobVersion = saved.Version
		return j
	}
	defer func() { _ = dkron.DeleteJob(ctx, "content", jobName) }()
	dkronSchedule := func() string {
		t.Helper()
		resp, err := http.Get("http://127.0.0.1:18082/v1/jobs/content-" + strings.ToLower(jobName))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var job struct {
			Schedule string `json:"schedule"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&job)
		return job.Schedule
	}

	// --- dev 晋级：r1 的任务经真实 Dkron 定时触发并成功 ---
	saveJob("@every 2s")
	r1 := accCall(t, mux, "POST", "projects/"+project+"/releases", token, map[string]string{"label": "r1", "code_ref": "sha-1"}, 201)
	accCall(t, mux, "POST", "projects/"+project+"/environments/"+dev+"/promote", token, map[string]any{"release_id": r1["id"]}, 200)
	deadline := time.Now().Add(25 * time.Second)
	fired := false
	for time.Now().Before(deadline) && !fired {
		list, _ := jobStore.List(ctx, project, "content", jobName, 20)
		for _, e := range list {
			if e.Trigger == jobs.TriggerScheduled && e.State == jobs.Succeeded {
				fired = true
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	if !fired {
		t.Fatal("promoted job did not fire through Dkron")
	}

	// --- protected 环境：服务端强制确认 ---
	accCall(t, mux, "POST", "projects/"+project+"/environments/"+prod+"/promote", token, map[string]any{"release_id": r1["id"]}, 409)
	accCall(t, mux, "POST", "projects/"+project+"/environments/"+prod+"/promote", token, map[string]any{"release_id": r1["id"], "confirm": true}, 200)

	// --- 部分失败注入：Dkron 停机时晋级失败，恢复后可重试 ---
	saveJob("0 0 1 1 *") // v2：远期调度
	r2 := accCall(t, mux, "POST", "projects/"+project+"/releases", token, map[string]string{"label": "r2", "code_ref": "sha-2"}, 201)
	exec.Command("docker", "stop", "mozi-v2-release-acc-dkron-1").Run()
	defer exec.Command("docker", "start", "mozi-v2-release-acc-dkron-1").Run()
	promoteResp := accCall(t, mux, "POST", "projects/"+project+"/environments/"+dev+"/promote", token, map[string]any{"release_id": r2["id"]}, 200)
	_ = promoteResp
	// 最新记录应为 failed。
	var failedState string
	if err := db.QueryRow(`SELECT state FROM environment_releases WHERE environment_id=$1 ORDER BY created_at DESC LIMIT 1`, dev).Scan(&failedState); err != nil {
		t.Fatal(err)
	}
	if failedState != "failed" {
		t.Fatalf("expected failed record during outage, got %s", failedState)
	}
	exec.Command("docker", "start", "mozi-v2-release-acc-dkron-1").Run()
	// Dkron's raft store accepts reads before it accepts writes; probe with
	// an actual create/delete round trip.
	deadline = time.Now().Add(60 * time.Second)
	for {
		probe := `{"name":"readiness-probe","schedule":"@every 1h","executor":"http","executor_config":{"method":"POST","url":"http://192.168.5.2:1/"}}`
		resp, err := http.Post("http://127.0.0.1:18082/v1/jobs", "application/json", strings.NewReader(probe))
		if err == nil && resp.StatusCode < 300 {
			resp.Body.Close()
			req, _ := http.NewRequest("DELETE", "http://127.0.0.1:18082/v1/jobs/readiness-probe", nil)
			dresp, derr := http.DefaultClient.Do(req)
			if derr == nil {
				dresp.Body.Close()
			}
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		if time.Now().After(deadline) {
			t.Fatal("dkron did not recover")
		}
		time.Sleep(500 * time.Millisecond)
	}
	recovery := accCall(t, mux, "POST", "projects/"+project+"/environments/"+dev+"/promote", token, map[string]any{"release_id": r2["id"]}, 200)
	if recovery["state"] != "ready" {
		t.Fatalf("recovery promote: %v", recovery)
	}
	// The adapter prepends seconds to the product-level 5-field cron.
	if got := dkronSchedule(); got != "0 0 0 1 1 *" {
		t.Fatalf("dkron schedule after r2: %q", got)
	}
	var r1State string
	if err := db.QueryRow(`SELECT state FROM environment_releases WHERE environment_id=$1 AND release_id=$2 AND action='promote' ORDER BY created_at LIMIT 1`, dev, r1["id"]).Scan(&r1State); err != nil {
		t.Fatal(err)
	}
	if r1State != "superseded" {
		t.Fatalf("r1 must be superseded on dev, got %s", r1State)
	}

	// --- 回退：重新应用 r1，Dkron 配置回到快照版本 ---
	rb := accCall(t, mux, "POST", "projects/"+project+"/environments/"+dev+"/rollback", token, map[string]any{}, 200)
	if rb["release_id"] != r1["id"] || rb["action"] != "rollback" || rb["state"] != "ready" {
		t.Fatalf("rollback: %v", rb)
	}
	if got := dkronSchedule(); got != "@every 2s" {
		t.Fatalf("dkron schedule after rollback: %q", got)
	}
	fmt.Println("promotion acceptance passed: promote, protected confirm, outage recovery, rollback")
}
