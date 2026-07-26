-- NATS account users (client credentials metadata) and sharing exports.

CREATE TABLE IF NOT EXISTS nats_account_users (
    id UUID PRIMARY KEY,
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    account_name TEXT NOT NULL DEFAULT 'Default',
    name TEXT NOT NULL,
    public_key TEXT NOT NULL DEFAULT '',
    seed_encrypted TEXT NOT NULL DEFAULT '',
    jwt TEXT NOT NULL DEFAULT '',
    signing_group TEXT NOT NULL DEFAULT 'Default',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (cluster_id, account_name, name)
);

CREATE INDEX IF NOT EXISTS idx_nats_account_users_cluster
    ON nats_account_users (cluster_id, account_name);

CREATE TABLE IF NOT EXISTS nats_account_exports (
    id UUID PRIMARY KEY,
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    account_name TEXT NOT NULL DEFAULT 'Default',
    kind TEXT NOT NULL CHECK (kind IN ('service', 'stream', 'feed')),
    name TEXT NOT NULL,
    subject TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (cluster_id, account_name, kind, name)
);

CREATE INDEX IF NOT EXISTS idx_nats_account_exports_cluster
    ON nats_account_exports (cluster_id, account_name);
