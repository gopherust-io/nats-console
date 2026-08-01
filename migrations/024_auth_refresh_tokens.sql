-- +goose Up
-- +goose StatementBegin
CREATE TABLE auth_refresh_tokens (
  id                UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
  user_id           TEXT NOT NULL,
  token_hash        TEXT NOT NULL UNIQUE,
  fingerprint_hash  TEXT NOT NULL,
  expires_at        TIMESTAMPTZ NOT NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  replaced_by       UUID NULL REFERENCES auth_refresh_tokens (id)
);

CREATE INDEX idx_auth_refresh_tokens_user_id ON auth_refresh_tokens (user_id);
CREATE INDEX idx_auth_refresh_tokens_expires_at ON auth_refresh_tokens (expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS auth_refresh_tokens;
-- +goose StatementEnd
