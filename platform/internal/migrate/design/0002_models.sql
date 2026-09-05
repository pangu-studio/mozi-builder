CREATE TABLE design_models (
    project_id TEXT NOT NULL REFERENCES design_projects(id),
    module TEXT NOT NULL,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    document JSONB NOT NULL,
    deleted BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(project_id,module,name),
    CHECK(module ~ '^[a-z][a-z0-9_]{0,62}$'),
    CHECK(name ~ '^[A-Z][A-Za-z0-9]{0,62}$'),
    CHECK(document->>'module'=module AND document->>'model'=name)
);
CREATE TABLE design_model_history (
    project_id TEXT NOT NULL,
    module TEXT NOT NULL,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    document JSONB NOT NULL,
    action TEXT NOT NULL CHECK(action IN ('created','updated','deleted')),
    actor_id TEXT NOT NULL,
    request_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(project_id,module,name,version),
    FOREIGN KEY(project_id,module,name) REFERENCES design_models(project_id,module,name)
);
COMMENT ON TABLE design_model_history IS 'Version snapshots and design change audit committed with the model in one design-database transaction.';
CREATE FUNCTION reject_design_history_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'design history is append-only'; END;
$$;
CREATE TRIGGER design_history_append_only BEFORE UPDATE OR DELETE ON design_model_history
    FOR EACH ROW EXECUTE FUNCTION reject_design_history_mutation();
