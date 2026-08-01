-- +goose Up
-- +goose StatementBegin
-- H2: persist auth session state so logout / permission invalidation is
-- enforced across all replicas, not just the process that issued it.

CREATE TABLE auth_user_versions (
  user_id     TEXT PRIMARY KEY,
  version     BIGINT NOT NULL DEFAULT 1,
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE auth_session_revocations (
  jti         TEXT PRIMARY KEY,
  expires_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_auth_session_revocations_expires_at ON auth_session_revocations (expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
