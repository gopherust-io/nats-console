CREATE TABLE IF NOT EXISTS clusters (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    nats_url TEXT NOT NULL,
    monitoring_url TEXT NOT NULL DEFAULT '',
    creds_file_path TEXT NOT NULL DEFAULT '',
    token TEXT NOT NULL DEFAULT '',
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_clusters_is_default ON clusters (is_default) WHERE is_default = TRUE;
