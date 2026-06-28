CREATE TABLE IF NOT EXISTS audit_log (
    id UUID PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor TEXT NOT NULL DEFAULT '',
    action TEXT NOT NULL,
    cluster_id TEXT NOT NULL DEFAULT '',
    resource_type TEXT NOT NULL DEFAULT '',
    resource_name TEXT NOT NULL DEFAULT '',
    request_id TEXT NOT NULL DEFAULT '',
    details JSONB NOT NULL DEFAULT '{}',
    ip TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_audit_log_timestamp ON audit_log (timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_cluster_id ON audit_log (cluster_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor ON audit_log (actor);
