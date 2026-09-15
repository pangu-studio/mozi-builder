package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
// fire endpoint; a disabled JobIR becomes a disabled Dkron job.
func (c DkronClient) SyncJob(ctx context.Context, j *mozi.JobIR, fireURL string) error {
	name := j.Module + "/" + j.Name
	job := dkronJob{
		Name:     name,
		Schedule: j.Schedule,
		Disabled: !j.IsEnabled(),
		Retries:  0, // business retries live in the platform, never in Dkron
		Executor: "http",
		ExecutorConfig: map[string]string{
			"method": "POST",
			"url":    fireURL,
		},
	}
	status, err := c.call(ctx, http.MethodPut, "jobs", job)
	if err != nil || status >= 300 {
		return fmt.Errorf("sync dkron job %s: status %d: %w", name, status, err)
	}
	return nil
}

// DeleteJob removes a job definition from Dkron, e.g. when the JobIR is
// deleted from the design database.
func (c DkronClient) DeleteJob(ctx context.Context, module, name string) error {
	status, err := c.call(ctx, http.MethodDelete, "jobs/"+module+"%2F"+name, nil)
	if err != nil || (status >= 300 && status != 404) {
		return fmt.Errorf("delete dkron job %s/%s: status %d: %w", module, name, status, err)
	}
	return nil
}
