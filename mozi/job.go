package mozi

// ============================================================================
// JobIR — scheduled business task definition (v2 phase 5)
// ============================================================================
//
// JobIR defines a scheduled business task. Dkron owns scheduling state; the
// platform owns execution protocol (execution_id per trigger, shared across
// retries of the same trigger). See docs/v2/jobs.md.

// JobIR is the intermediate representation of one scheduled task.
type JobIR struct {
	SchemaVersion            int           `yaml:"schema_version,omitempty" json:"schema_version"`
	Module                   string        `yaml:"module" json:"module"`
	Name                     string        `yaml:"job" json:"job"` // PascalCase, e.g. DeckDigestJob
	Label                    string        `yaml:"label" json:"label"`
	Description              string        `yaml:"description,omitempty" json:"description,omitempty"`
	Schedule                 string        `yaml:"schedule" json:"schedule"` // 5-field cron or "@every 30s"
	Executor                 JobExecutorIR `yaml:"executor" json:"executor"`
	TimeoutSeconds           int           `yaml:"timeout_seconds" json:"timeout_seconds"`
	Retry                    JobRetryIR    `yaml:"retry" json:"retry"`
	LongRunning              bool          `yaml:"long_running,omitempty" json:"long_running,omitempty"`
	HeartbeatIntervalSeconds int           `yaml:"heartbeat_interval_seconds,omitempty" json:"heartbeat_interval_seconds,omitempty"`
	Enabled                  *bool         `yaml:"enabled,omitempty" json:"enabled,omitempty"` // nil means true
}

// JobExecutorIR describes how the task is invoked. Currently only HTTP
// endpoints behind the gateway are supported.
type JobExecutorIR struct {
	Kind   string `yaml:"kind" json:"kind"`     // http
	Method string `yaml:"method" json:"method"` // POST | PUT
	Path   string `yaml:"path" json:"path"`     // business execution endpoint, e.g. /jobs/deck-digest
}

// JobRetryIR is the business retry policy. Dkron-level retries stay disabled
// so retry semantics live in exactly one place.
type JobRetryIR struct {
	MaxAttempts    int `yaml:"max_attempts" json:"max_attempts"`
	BackoffSeconds int `yaml:"backoff_seconds" json:"backoff_seconds"` // fixed interval
}

// IsEnabled reports whether the job participates in scheduling.
func (j *JobIR) IsEnabled() bool {
	return j.Enabled == nil || *j.Enabled
}
