-- +goose Up
-- +goose StatementBegin
-- RANGE-partition high-churn sample tables by day; drop unused surrogate UUID PKs.

ALTER TABLE cluster_metric_samples RENAME TO cluster_metric_samples_old;
ALTER INDEX IF EXISTS idx_cluster_metric_samples_range RENAME TO idx_cluster_metric_samples_range_old;

CREATE TABLE cluster_metric_samples (
  cluster_id  UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  captured_at TIMESTAMPTZ NOT NULL,
  metric      TEXT NOT NULL,
  value       DOUBLE PRECISION NOT NULL,
  PRIMARY KEY (cluster_id, captured_at, metric)
) PARTITION BY RANGE (captured_at);

CREATE INDEX idx_cluster_metric_samples_range
  ON cluster_metric_samples (cluster_id, metric, captured_at DESC);

ALTER TABLE incident_consumer_samples RENAME TO incident_consumer_samples_old;
ALTER INDEX IF EXISTS idx_incident_consumer_samples_lookup RENAME TO idx_incident_consumer_samples_lookup_old;

CREATE TABLE incident_consumer_samples (
  cluster_id       UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  captured_at      TIMESTAMPTZ NOT NULL,
  stream_name      TEXT NOT NULL,
  consumer_name    TEXT NOT NULL,
  lag              DOUBLE PRECISION NOT NULL DEFAULT 0,
  num_redelivered  DOUBLE PRECISION NOT NULL DEFAULT 0,
  delivered_seq    DOUBLE PRECISION NOT NULL DEFAULT 0,
  ack_floor_seq    DOUBLE PRECISION NOT NULL DEFAULT 0,
  PRIMARY KEY (cluster_id, captured_at, stream_name, consumer_name)
) PARTITION BY RANGE (captured_at);

CREATE INDEX idx_incident_consumer_samples_lookup
  ON incident_consumer_samples (cluster_id, stream_name, consumer_name, captured_at DESC);

-- Daily partitions covering existing data + retention window + forward buffer (UTC).
DO $$
DECLARE
  d date;
  end_d date;
  min_d date;
  max_d date;
  today date := (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')::date;
  part_name text;
  from_ts timestamptz;
  to_ts timestamptz;
BEGIN
  SELECT
    COALESCE(MIN((captured_at AT TIME ZONE 'UTC')::date), today - 8),
    COALESCE(MAX((captured_at AT TIME ZONE 'UTC')::date), today + 2)
  INTO min_d, max_d
  FROM (
    SELECT captured_at FROM cluster_metric_samples_old
    UNION ALL
    SELECT captured_at FROM incident_consumer_samples_old
  ) t;

  d := LEAST(min_d, today - 8);
  end_d := GREATEST(max_d, today + 2);

  WHILE d <= end_d LOOP
    part_name := to_char(d, 'YYYY_MM_DD');
    from_ts := d::timestamp AT TIME ZONE 'UTC';
    to_ts := (d + 1)::timestamp AT TIME ZONE 'UTC';
    EXECUTE format(
      'CREATE TABLE IF NOT EXISTS cluster_metric_samples_%s PARTITION OF cluster_metric_samples FOR VALUES FROM (%L) TO (%L)',
      part_name, from_ts, to_ts
    );
    EXECUTE format(
      'CREATE TABLE IF NOT EXISTS incident_consumer_samples_%s PARTITION OF incident_consumer_samples FOR VALUES FROM (%L) TO (%L)',
      part_name, from_ts, to_ts
    );
    d := d + 1;
  END LOOP;
END $$;

INSERT INTO cluster_metric_samples (cluster_id, captured_at, metric, value)
SELECT cluster_id, captured_at, metric, value FROM cluster_metric_samples_old;

INSERT INTO incident_consumer_samples (
  cluster_id, captured_at, stream_name, consumer_name,
  lag, num_redelivered, delivered_seq, ack_floor_seq
)
SELECT
  cluster_id, captured_at, stream_name, consumer_name,
  lag, num_redelivered, delivered_seq, ack_floor_seq
FROM incident_consumer_samples_old;

DROP TABLE cluster_metric_samples_old;
DROP TABLE incident_consumer_samples_old;

-- Time-leading indexes for low-volume incident cleanup DELETEs.
CREATE INDEX IF NOT EXISTS idx_incident_node_events_occurred_at
  ON incident_node_events (occurred_at);
CREATE INDEX IF NOT EXISTS idx_incident_annotations_occurred_at
  ON incident_annotations (occurred_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
