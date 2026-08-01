-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS request_reply_probes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    subject TEXT NOT NULL,
    payload BYTEA NOT NULL DEFAULT ''::bytea,
    timeout_ms INT NOT NULL DEFAULT 2000,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (cluster_id, subject)
);

CREATE INDEX IF NOT EXISTS idx_request_reply_probes_cluster
    ON request_reply_probes (cluster_id, enabled);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
