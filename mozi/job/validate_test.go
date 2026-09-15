package job

import (
	"strings"
	"testing"

	"github.com/pangu-studio/mozi-builder/mozi"
)

func validJob() *mozi.JobIR {
	return &mozi.JobIR{
		Module:         "content",
		Name:           "DeckDigestJob",
		Label:          "牌组摘要任务",
		Schedule:       "0 3 * * *",
		Executor:       mozi.JobExecutorIR{Kind: "http", Method: "POST", Path: "/jobs/deck-digest"},
		TimeoutSeconds: 300,
		Retry:          mozi.JobRetryIR{MaxAttempts: 3, BackoffSeconds: 60},
	}
}

func TestValidateValidJob(t *testing.T) {
	if res := Validate(validJob()); !res.Valid {
		t.Fatalf("expected valid: %v", res.Errors)
	}
}

func TestValidateEverySchedule(t *testing.T) {
	j := validJob()
	j.Schedule = "@every 30s"
	if res := Validate(j); !res.Valid {
		t.Fatalf("expected valid: %v", res.Errors)
	}
	j.Schedule = "@every not-a-duration"
	assertError(t, Validate(j), "invalid @every duration")
}

func TestValidateSchedules(t *testing.T) {
	for _, s := range []string{"", "@daily", "0 3 * *", "0 3 * * * *", "ab cd ef gh ij"} {
		j := validJob()
		j.Schedule = s
		if res := Validate(j); res.Valid {
			t.Fatalf("schedule %q must be rejected", s)
		}
	}
}

func TestValidateExecutor(t *testing.T) {
	j := validJob()
	j.Executor.Method = "GET"
	assertError(t, Validate(j), "method must be POST or PUT")
	j = validJob()
	j.Executor.Path = "jobs/x"
	assertError(t, Validate(j), "must start with /")
	j = validJob()
	j.Executor.Kind = "grpc"
	assertError(t, Validate(j), "unsupported executor kind")
}

func TestValidateRetryAndTimeout(t *testing.T) {
	j := validJob()
	j.TimeoutSeconds = 0
	assertError(t, Validate(j), "timeout")
	j = validJob()
	j.Retry.MaxAttempts = 0
	assertError(t, Validate(j), "max_attempts")
	j = validJob()
	j.Retry.BackoffSeconds = 0
	assertError(t, Validate(j), "backoff_seconds")
}

func TestValidateLongRunning(t *testing.T) {
	j := validJob()
	j.LongRunning = true
	assertError(t, Validate(j), "heartbeat")
	j.HeartbeatIntervalSeconds = 30
	res := Validate(j)
	if !res.Valid {
		t.Fatalf("expected valid: %v", res.Errors)
	}
	j.HeartbeatIntervalSeconds = 999
	res = Validate(j)
	found := false
	for _, w := range res.Warnings {
		if strings.Contains(w.Message, "shorter than the attempt timeout") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected heartbeat/timeout warning")
	}
}

func TestDisabledJobWarning(t *testing.T) {
	j := validJob()
	off := false
	j.Enabled = &off
	res := Validate(j)
	if !res.Valid {
		t.Fatal("disabled job stays valid")
	}
	if j.IsEnabled() {
		t.Fatal("IsEnabled must honor explicit false")
	}
	if len(res.Warnings) == 0 {
		t.Fatal("expected disabled scheduling warning")
	}
	if !validJob().IsEnabled() {
		t.Fatal("nil Enabled must mean true")
	}
}

func assertError(t *testing.T, res *ValidationResult, substr string) {
	t.Helper()
	if res.Valid {
		t.Fatalf("expected invalid containing %q", substr)
	}
	for _, e := range res.Errors {
		if strings.Contains(e.Message, substr) {
			return
		}
	}
	t.Fatalf("no error containing %q: %v", substr, res.Errors)
}
