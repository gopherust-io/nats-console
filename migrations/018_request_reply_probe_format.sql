-- +goose Up
-- +goose StatementBegin
ALTER TABLE request_reply_probes
  ADD COLUMN IF NOT EXISTS payload_format TEXT NOT NULL DEFAULT 'json';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
