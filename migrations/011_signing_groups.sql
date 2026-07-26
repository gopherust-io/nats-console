-- Signing key groups for NATS account users (Phase 2).

CREATE TABLE IF NOT EXISTS nats_signing_groups (
    id UUID PRIMARY KEY,
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    account_name TEXT NOT NULL DEFAULT 'Default',
    name TEXT NOT NULL,
    scoped BOOLEAN NOT NULL DEFAULT false,
    pub_allow TEXT[] NOT NULL DEFAULT '{}',
    pub_deny TEXT[] NOT NULL DEFAULT '{}',
    sub_allow TEXT[] NOT NULL DEFAULT '{}',
    sub_deny TEXT[] NOT NULL DEFAULT '{}',
    max_data BIGINT NOT NULL DEFAULT -1,
    max_payload BIGINT NOT NULL DEFAULT -1,
    max_subs BIGINT NOT NULL DEFAULT -1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (cluster_id, account_name, name)
);

CREATE INDEX IF NOT EXISTS idx_nats_signing_groups_cluster
    ON nats_signing_groups (cluster_id, account_name);

ALTER TABLE nats_account_users
    ADD COLUMN IF NOT EXISTS assigned_user_id UUID REFERENCES users(id) ON DELETE SET NULL;
