-- +goose Up
-- +goose StatementBegin
CREATE TABLE alert_rules (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  cluster_id    UUID REFERENCES clusters(id) ON DELETE CASCADE,
  account_name  TEXT,
  name          TEXT NOT NULL,
  message       TEXT NOT NULL DEFAULT '',
  severity      TEXT NOT NULL CHECK (severity IN ('info', 'warning', 'critical')),
  metric        TEXT NOT NULL,
  comparator    TEXT NOT NULL CHECK (comparator IN ('gt', 'gte', 'lt', 'lte')),
  threshold     DOUBLE PRECISION NOT NULL,
  enabled       BOOLEAN NOT NULL DEFAULT true,
  created_by    TEXT NOT NULL DEFAULT '',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_alert_rules_enabled ON alert_rules (enabled) WHERE enabled = true;
CREATE INDEX idx_alert_rules_cluster ON alert_rules (cluster_id);

CREATE TABLE alerts (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  rule_id          UUID NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
  cluster_id       UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  account_name     TEXT,
  status           TEXT NOT NULL CHECK (status IN ('open', 'closed')),
  severity         TEXT NOT NULL CHECK (severity IN ('info', 'warning', 'critical')),
  metric           TEXT NOT NULL,
  message          TEXT NOT NULL DEFAULT '',
  firing_value     DOUBLE PRECISION NOT NULL,
  threshold        DOUBLE PRECISION NOT NULL,
  first_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  closed_at        TIMESTAMPTZ,
  acknowledged_at  TIMESTAMPTZ,
  acknowledged_by  TEXT
);

CREATE UNIQUE INDEX idx_alerts_open_rule_cluster
  ON alerts (rule_id, cluster_id) WHERE status = 'open';

CREATE INDEX idx_alerts_status_last_seen ON alerts (status, last_seen_at DESC);
CREATE INDEX idx_alerts_cluster_status ON alerts (cluster_id, status);
CREATE INDEX idx_alerts_unacked_open
  ON alerts (last_seen_at DESC) WHERE status = 'open' AND acknowledged_at IS NULL;

-- Seeded defaults (disabled until an admin enables them)
INSERT INTO alert_rules (name, message, severity, metric, comparator, threshold, enabled, created_by)
VALUES
  (
    'High CPU',
    'Server CPU usage exceeded threshold',
    'warning',
    'server.cpu_percent',
    'gte',
    90,
    false,
    'system'
  ),
  (
    'High connections',
    'Server connection count exceeded threshold',
    'warning',
    'server.connections',
    'gte',
    10000,
    false,
    'system'
  ),
  (
    'High JetStream storage',
    'JetStream file storage exceeded threshold',
    'critical',
    'jetstream.storage_bytes',
    'gte',
    107374182400,
    false,
    'system'
  );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
