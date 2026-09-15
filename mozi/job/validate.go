// Package job provides validation for mozi.JobIR documents.
// See docs/v2/jobs.md for the business task protocol.
package job

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pangu-studio/mozi-builder/mozi"
)

var (
	pascalCaseRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	moduleNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	cronFieldRe  = regexp.MustCompile(`^[\d*/,?\-LW#]+$`)
	everyRe      = regexp.MustCompile(`^@every\s+(\S+)$`)
)

// ValidationError represents a single JobIR validation issue.
type ValidationError struct {
	Job     string `json:"job"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("[%s.%s] %s", e.Job, e.Field, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Job, e.Message)
}

// ValidationResult holds the result of validating a JobIR.
type ValidationResult struct {
	Valid    bool               `json:"valid"`
	Errors   []*ValidationError `json:"errors,omitempty"`
	Warnings []*ValidationError `json:"warnings,omitempty"`
}

// Validate checks a JobIR for structural correctness.
func Validate(j *mozi.JobIR) *ValidationResult {
	result := &ValidationResult{Valid: true}
	ref := j.Module + "/" + j.Name
	add := func(field, msg string) {
		result.Errors = append(result.Errors, &ValidationError{Job: ref, Field: field, Message: msg})
		result.Valid = false
	}
	warn := func(field, msg string) {
		result.Warnings = append(result.Warnings, &ValidationError{Job: ref, Field: field, Message: msg})
	}

	if j.Name == "" {
		add("", "job name is required")
	} else if !pascalCaseRe.MatchString(j.Name) {
		add("", fmt.Sprintf("job name must be PascalCase: %s", j.Name))
	}
	if j.Module == "" {
		add("", "module is required")
	} else if !moduleNameRe.MatchString(j.Module) {
		add("", fmt.Sprintf("module must be snake_case: %s", j.Module))
	}
	if strings.TrimSpace(j.Label) == "" {
		add("", "label is required")
	}

	validateSchedule(j.Schedule, add)

	if j.Executor.Kind != "http" {
		add("executor", fmt.Sprintf("unsupported executor kind %q (only http)", j.Executor.Kind))
	}
	if j.Executor.Method != "POST" && j.Executor.Method != "PUT" {
		add("executor", fmt.Sprintf("method must be POST or PUT: %s", j.Executor.Method))
	}
	if !strings.HasPrefix(j.Executor.Path, "/") {
		add("executor", fmt.Sprintf("path must start with /: %q", j.Executor.Path))
	}

	if j.TimeoutSeconds < 1 {
		add("timeout_seconds", "timeout must be at least 1 second")
	}
	if j.Retry.MaxAttempts < 1 {
		add("retry", "max_attempts must be at least 1")
	}
	if j.Retry.MaxAttempts > 1 && j.Retry.BackoffSeconds < 1 {
		add("retry", "backoff_seconds must be at least 1 when retrying")
	}
	if j.LongRunning {
		if j.HeartbeatIntervalSeconds < 1 {
			add("heartbeat_interval_seconds", "long_running jobs require a heartbeat interval")
		} else if j.TimeoutSeconds >= 1 && j.HeartbeatIntervalSeconds >= j.TimeoutSeconds {
			warn("heartbeat_interval_seconds", "heartbeat interval should be shorter than the attempt timeout")
		}
	}
	if !j.IsEnabled() {
		warn("", "disabled jobs are not scheduled and cannot be run-triggered; manual execution creates an ad-hoc execution instead")
	}
	return result
}

// validateSchedule accepts a 5-field cron expression or "@every <duration>".
func validateSchedule(schedule string, add func(field, msg string)) {
	s := strings.TrimSpace(schedule)
	if s == "" {
		add("schedule", "schedule is required")
		return
	}
	if m := everyRe.FindStringSubmatch(s); m != nil {
		if _, err := time.ParseDuration(m[1]); err != nil {
			add("schedule", fmt.Sprintf("invalid @every duration %q", m[1]))
		}
		return
	}
	if strings.HasPrefix(s, "@") {
		add("schedule", fmt.Sprintf("unsupported schedule macro %q (use 5-field cron or @every)", s))
		return
	}
	fields := strings.Fields(s)
	if len(fields) != 5 {
		add("schedule", fmt.Sprintf("cron expression must have 5 fields, got %d", len(fields)))
		return
	}
	for i, f := range fields {
		if !cronFieldRe.MatchString(f) {
			add("schedule", fmt.Sprintf("invalid cron field %d: %q", i+1, f))
		}
	}
}
