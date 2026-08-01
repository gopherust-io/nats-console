-- +goose Up
-- +goose StatementBegin
ALTER TABLE alerts
  ADD COLUMN IF NOT EXISTS email_notified_at TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
