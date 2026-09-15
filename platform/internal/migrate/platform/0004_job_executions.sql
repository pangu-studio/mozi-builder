CREATE TABLE job_executions (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id),
    module TEXT NOT NULL,
    job TEXT NOT NULL,
    execution_id TEXT NOT NULL UNIQUE,
    trigger TEXT NOT NULL CHECK(trigger IN ('scheduled','manual','retry')),
    attempt INT NOT NULL CHECK(attempt >= 1),
    state TEXT NOT NULL CHECK(state IN ('running','succeeded','failed','lost')),
    error TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    heartbeat_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE job_executions IS 'Business task protocol: one row per execution_id; retries of the same trigger share execution_id with incrementing attempt.';
CREATE INDEX job_executions_running ON job_executions(state) WHERE state='running';
CREATE INDEX job_executions_by_job ON job_executions(project_id,module,job,created_at DESC);
