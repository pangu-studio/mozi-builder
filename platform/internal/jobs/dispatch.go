package jobs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/pangu-studio/mozi-builder/mozi"
)

// Protocol headers carried to the business executor.
const (
	HeaderExecutionID = "X-Mozi-Execution-Id"
	HeaderAttempt     = "X-Mozi-Attempt"
	HeaderJob         = "X-Mozi-Job"
	HeaderTrigger     = "X-Mozi-Trigger"
)

// Dispatcher invokes the business executor for one attempt. A timeout marks
// the attempt failed; it never terminates the business work itself.
type Dispatcher struct {
	Store      Store
	GatewayURL string
	Client     *http.Client
}

func (d Dispatcher) client(timeout time.Duration) *http.Client {
	if d.Client != nil {
		return d.Client
	}
	return &http.Client{Timeout: timeout}
}

// Dispatch executes one attempt of the execution and records the outcome.
func (d Dispatcher) Dispatch(ctx context.Context, e Execution, j *mozi.JobIR) error {
	timeout := time.Duration(j.TimeoutSeconds) * time.Second
	url := d.GatewayURL + j.Executor.Path
	req, err := http.NewRequestWithContext(ctx, j.Executor.Method, url, bytes.NewReader(nil))
	if err != nil {
		return err
	}
	req.Header.Set(HeaderExecutionID, e.ExecutionID)
	req.Header.Set(HeaderAttempt, strconv.Itoa(e.Attempt))
	req.Header.Set(HeaderJob, j.Module+"/"+j.Name)
	req.Header.Set(HeaderTrigger, e.Trigger)
	resp, err := d.client(timeout).Do(req)
	if err != nil {
		if cerr := d.Store.Complete(ctx, e.ExecutionID, e.Attempt, false, err.Error()); cerr != nil {
			return cerr
		}
		return fmt.Errorf("dispatch %s attempt %d: %w", e.ExecutionID, e.Attempt, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return d.Store.Complete(ctx, e.ExecutionID, e.Attempt, true, "")
	}
	return d.Store.Complete(ctx, e.ExecutionID, e.Attempt, false, fmt.Sprintf("executor returned %d", resp.StatusCode))
}
