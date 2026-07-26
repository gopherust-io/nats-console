-- Console access grants and people invite tokens.

CREATE TABLE IF NOT EXISTS access_grants (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    resource_type TEXT NOT NULL CHECK (resource_type IN ('system', 'account', 'nats_user')),
    resource_key TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'observer', 'credential_downloader')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, resource_type, resource_key)
);

CREATE INDEX IF NOT EXISTS idx_access_grants_user ON access_grants (user_id);
CREATE INDEX IF NOT EXISTS idx_access_grants_resource ON access_grants (resource_type, resource_key);

CREATE TABLE IF NOT EXISTS user_invites (
    token TEXT PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_invites_user ON user_invites (user_id);
