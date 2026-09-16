package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pangu-studio/mozi-builder/mozi"
)

// DkronClient synchronizes JobIR definitions to Dkron. Dkron owns scheduling
// only: business retries stay in the platform, so Dkron-level retries are
// always zero. Disabled jobs are never run-triggered (Dkron 4.1.3 behavior);
// manual execution creates an ad-hoc platform execution instead.
type DkronClient struct {
	BaseURL string
	Client  *http.Client
}

type dkronJob struct {
	Name           string            `json:"name"`
	Schedule       string            `json:"schedule"`
	Disabled       bool              `json:"disabled"`
	Retries        int               `json:"retries"`
	Executor       string            `json:"executor"`
	ExecutorConfig map[string]string `json:"executor_config"`
}

func (c DkronClient) call(ctx context.Context, method, path string, body any) (int, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+"/v1/"+path, reader)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return resp.StatusCode, nil
}

// SyncJob upserts an enabled JobIR as a Dkron job pointing at the platform
// fire endpoint; a disabled JobIR becomes a disabled Dkron job. fireKey is
// sent as the shared-secret header on every trigger. Dkron 4.1.3 POST only
// creates (repeated posts do not update), so sync deletes first to stay
// idempotent.
func (c DkronClient) SyncJob(ctx context.Context, j *mozi.JobIR, fireURL, fireKey string) error {
	if err := c.DeleteJob(ctx, j.Module, j.Name); err != nil {
		return err
	}
	name := JobName(j)
	config := map[string]string{
		"method": "POST",
		"url":    fireURL,
	}
	// Dkron 4.1.3 cron has six fields (seconds first); JobIR uses the
	// product-level five-field form, so the adapter prepends seconds.
	schedule := j.Schedule
	if fields := strings.Fields(schedule); len(fields) == 5 {
		schedule = "0 " + schedule
	}
	// Dkron 4.1.3 does not deliver custom executor headers (verified
	// against the acceptance Dkron), so the shared secret travels as a
	// fire_key query parameter on the internal network.
	if fireKey != "" {
		sep := "?"
		if strings.Contains(fireURL, "?") {
			sep = "&"
		}
		config["url"] = fireURL + sep + "fire_key=" + url.QueryEscape(fireKey)
	}
	job := dkronJob{
		Name:           name,
		Schedule:       schedule,
		Disabled:       !j.IsEnabled(),
		Retries:        0, // business retries live in the platform, never in Dkron
		Executor:       "http",
		ExecutorConfig: config,
	}
	status, err := c.call(ctx, http.MethodPost, "jobs", job)
	if err != nil || status >= 300 {
		return fmt.Errorf("sync dkron job %s: status %d: %w", name, status, err)
	}
	return nil
}

// DeleteJob removes a job definition from Dkron, e.g. when the JobIR is
// deleted from the design database.
func (c DkronClient) DeleteJob(ctx context.Context, module, name string) error {
	status, err := c.call(ctx, http.MethodDelete, "jobs/"+module+"-"+strings.ToLower(name), nil)
	if err != nil || (status >= 300 && status != 404) {
		return fmt.Errorf("delete dkron job %s-%s: status %d: %w", module, name, status, err)
	}
	return nil
}

// JobName is the Dkron-side identifier for a JobIR. Dkron 4.1.3 rejects
// slashes, dots, and uppercase letters (verified against the acceptance
// Dkron), so the identifier is lowercase with a hyphen separator. JobIR
// names are PascalCase, so lowercasing stays unique within a module.
func JobName(j *mozi.JobIR) string {
	return j.Module + "-" + strings.ToLower(j.Name)
}
