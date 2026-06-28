CREATE TABLE cluster_metric_samples (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  cluster_id  UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  captured_at TIMESTAMPTZ NOT NULL,
  metric      TEXT NOT NULL,
  value       DOUBLE PRECISION NOT NULL,
  UNIQUE (cluster_id, captured_at, metric)
);

CREATE INDEX idx_cluster_metric_samples_range
  ON cluster_metric_samples (cluster_id, metric, captured_at DESC);
