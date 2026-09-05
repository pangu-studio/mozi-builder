CREATE TABLE design_projects (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (id <> ''),
    CHECK (slug ~ '^[a-z][a-z0-9-]{1,62}$'),
    CHECK (name <> '')
);

COMMENT ON TABLE design_projects IS 'Project scope for v2 design data; linked by stable text ID to the platform database.';
