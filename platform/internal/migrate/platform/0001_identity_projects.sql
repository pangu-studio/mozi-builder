CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    disabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (id <> ''), CHECK (email = lower(email)), CHECK (display_name <> ''), CHECK (password_hash <> '')
);
CREATE UNIQUE INDEX users_email_unique ON users (lower(email));

CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (id <> ''), CHECK (slug ~ '^[a-z][a-z0-9-]{1,62}$'), CHECK (name <> '')
);

CREATE TABLE project_members (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, user_id),
    CHECK (role IN ('owner', 'maintainer', 'developer', 'viewer'))
);
CREATE INDEX project_members_user_idx ON project_members(user_id, project_id);

CREATE TABLE environments (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    protected BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, slug),
    CHECK (id <> ''), CHECK (slug ~ '^[a-z][a-z0-9-]{1,62}$'),
    CHECK (name <> ''), CHECK (kind IN ('development', 'staging', 'production'))
);

CREATE TABLE audit_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    project_id TEXT REFERENCES projects(id) ON DELETE SET NULL,
    environment_id TEXT REFERENCES environments(id) ON DELETE SET NULL,
    request_id TEXT NOT NULL,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    result TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    CHECK (request_id <> ''), CHECK (action <> ''), CHECK (resource_type <> ''),
    CHECK (resource_id <> ''), CHECK (result IN ('succeeded', 'denied', 'failed'))
);
CREATE INDEX audit_events_project_time_idx ON audit_events(project_id, occurred_at DESC);
CREATE INDEX audit_events_actor_time_idx ON audit_events(actor_id, occurred_at DESC);

COMMENT ON TABLE audit_events IS 'Append-only platform audit trail. Application roles must not receive UPDATE or DELETE grants.';
