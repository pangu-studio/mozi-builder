CREATE TABLE release_operations (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id),
    resource TEXT NOT NULL,
    kind TEXT NOT NULL CHECK(kind IN ('deploy','scale','route','disable')),
    desired JSONB NOT NULL,
    idempotency_key TEXT NOT NULL UNIQUE,
    state TEXT NOT NULL CHECK(state IN ('Pending','Applying','Ready','Failed','Drifted')),
    attempts INT NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    actor_id TEXT NOT NULL,
    request_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Single config manager: at most one active operation per resource.
CREATE UNIQUE INDEX release_operations_active_resource
    ON release_operations(project_id, resource)
    WHERE state IN ('Pending','Applying');
CREATE TABLE release_readbacks (
    id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL REFERENCES release_operations(id),
    observed JSONB NOT NULL,
    match BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE release_readbacks IS 'Read-back verification snapshots; append-only audit trail for release state transitions and drift detection.';
CREATE TRIGGER release_readbacks_append_only BEFORE UPDATE OR DELETE ON release_readbacks
    FOR EACH ROW EXECUTE FUNCTION reject_audit_mutation();
