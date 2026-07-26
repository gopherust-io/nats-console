-- Extended NATS user JWT fields beyond Synadia create-panel parity.

ALTER TABLE nats_account_users
    ADD COLUMN IF NOT EXISTS bearer_token BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS proxy_required BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS allowed_connection_types TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS src_cidrs TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS times_locale TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS time_ranges JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS resp_max_msgs INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS resp_ttl_ns BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_data BIGINT NOT NULL DEFAULT -1;
