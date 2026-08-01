-- +goose Up
-- +goose StatementBegin
-- Compact hourly rollups for hidden-bottleneck schedule mining (~28d retention).

CREATE TABLE IF NOT EXISTS bottleneck_hour_rollups (
  cluster_id          UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  stream_name         TEXT NOT NULL,
  consumer_name       TEXT NOT NULL DEFAULT '',
  bucket_hour         TIMESTAMPTZ NOT NULL,
  avg_lag             DOUBLE PRECISION NOT NULL DEFAULT 0,
  max_lag             DOUBLE PRECISION NOT NULL DEFAULT 0,
  avg_payload_bytes   DOUBLE PRECISION NOT NULL DEFAULT 0,
  avg_processing_ms   DOUBLE PRECISION,
  samples             INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (cluster_id, stream_name, consumer_name, bucket_hour)
);

CREATE INDEX IF NOT EXISTS idx_bottleneck_hour_rollups_cluster_time
  ON bottleneck_hour_rollups (cluster_id, bucket_hour DESC);

CREATE INDEX IF NOT EXISTS idx_bottleneck_hour_rollups_lookup
  ON bottleneck_hour_rollups (cluster_id, stream_name, consumer_name, bucket_hour DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
