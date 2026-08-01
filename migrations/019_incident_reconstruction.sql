-- +goose Up
-- +goose StatementBegin
-- Incident reconstruction: deploy annotations, per-consumer samples, and node transitions.

CREATE TABLE IF NOT EXISTS incident_annotations (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  cluster_id  UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  occurred_at TIMESTAMPTZ NOT NULL,
  type        TEXT NOT NULL DEFAULT 'deploy',
  title       TEXT NOT NULL DEFAULT '',
  details     TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_incident_annotations_cluster_time
  ON incident_annotations (cluster_id, occurred_at DESC);

CREATE TABLE IF NOT EXISTS incident_consumer_samples (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  cluster_id       UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  captured_at      TIMESTAMPTZ NOT NULL,
  stream_name      TEXT NOT NULL,
  consumer_name    TEXT NOT NULL,
  lag              DOUBLE PRECISION NOT NULL DEFAULT 0,
  num_redelivered  DOUBLE PRECISION NOT NULL DEFAULT 0,
  delivered_seq    DOUBLE PRECISION NOT NULL DEFAULT 0,
  ack_floor_seq    DOUBLE PRECISION NOT NULL DEFAULT 0,
  UNIQUE (cluster_id, captured_at, stream_name, consumer_name)
);

CREATE INDEX IF NOT EXISTS idx_incident_consumer_samples_lookup
  ON incident_consumer_samples (cluster_id, stream_name, consumer_name, captured_at DESC);

CREATE TABLE IF NOT EXISTS incident_node_events (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  cluster_id  UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  occurred_at TIMESTAMPTZ NOT NULL,
  node_name   TEXT NOT NULL,
  event_type  TEXT NOT NULL CHECK (event_type IN ('disconnect', 'reconnect')),
  UNIQUE (cluster_id, occurred_at, node_name, event_type)
);

CREATE INDEX IF NOT EXISTS idx_incident_node_events_cluster_time
  ON incident_node_events (cluster_id, occurred_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
