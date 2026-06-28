-- JWT resolver account imports (encrypted at rest).

CREATE TABLE IF NOT EXISTS nats_jwt_accounts (
    id UUID PRIMARY KEY,
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    jwt TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (cluster_id, name)
);

CREATE INDEX IF NOT EXISTS idx_nats_jwt_accounts_cluster ON nats_jwt_accounts (cluster_id);
