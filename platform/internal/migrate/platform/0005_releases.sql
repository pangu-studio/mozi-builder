CREATE TABLE releases (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id),
    label TEXT NOT NULL,
    design_versions JSONB NOT NULL,
    code_ref TEXT NOT NULL,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK(label <> ''),
    CHECK(code_ref <> '')
);
COMMENT ON COLUMN releases.design_versions IS 'Frozen snapshot of design collection version tokens (models/services/jobs) taken at release creation; later design changes never mutate it.';
CREATE INDEX releases_by_project ON releases(project_id,created_at DESC);
CREATE TABLE environment_releases (
    environment_id TEXT NOT NULL REFERENCES environments(id),
    release_id TEXT NOT NULL REFERENCES releases(id),
    action TEXT NOT NULL CHECK(action IN ('promote','rollback')),
    state TEXT NOT NULL CHECK(state IN ('applying','ready','failed','superseded')),
    actor_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(environment_id,release_id,action,created_at)
);
COMMENT ON TABLE environment_releases IS 'Append-only promotion/rollback history per environment; superseded marks a previously ready release that a newer promotion replaced.';
