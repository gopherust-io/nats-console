-- +goose Up
-- +goose StatementBegin
-- Align indexes with store query patterns; drop redundant indexes.

CREATE INDEX IF NOT EXISTS idx_audit_log_cluster_time
  ON audit_log (cluster_id, timestamp DESC);
DROP INDEX IF EXISTS idx_audit_log_actor;
DROP INDEX IF EXISTS idx_audit_log_cluster_id;

CREATE INDEX IF NOT EXISTS idx_alerts_cluster_status_seen
  ON alerts (cluster_id, status, last_seen_at DESC);

DROP INDEX IF EXISTS idx_access_grants_user;
DROP INDEX IF EXISTS idx_nats_account_users_cluster;
DROP INDEX IF EXISTS idx_nats_account_exports_cluster;
DROP INDEX IF EXISTS idx_nats_signing_groups_cluster;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
