-- +goose Up
-- +goose StatementBegin
-- Daily architecture scores for Docs score card + multi-month trend (~180d retention).

CREATE TABLE IF NOT EXISTS architecture_score_daily (
  cluster_id   UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  score_day    DATE NOT NULL,
  score        INTEGER NOT NULL CHECK (score >= 0 AND score <= 100),
  factors      JSONB NOT NULL DEFAULT '[]'::jsonb,
  avg_lag      DOUBLE PRECISION NOT NULL DEFAULT 0,
  captured_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (cluster_id, score_day)
);

CREATE INDEX IF NOT EXISTS idx_architecture_score_daily_cluster_day
  ON architecture_score_daily (cluster_id, score_day DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
