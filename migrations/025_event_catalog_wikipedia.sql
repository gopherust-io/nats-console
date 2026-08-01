-- +goose Up
-- +goose StatementBegin
ALTER TABLE event_catalog_entries
  ADD COLUMN IF NOT EXISTS example JSONB,
  ADD COLUMN IF NOT EXISTS deprecated BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS successor_subject TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS deprecation_note TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE event_catalog_entries
  DROP COLUMN IF EXISTS example,
  DROP COLUMN IF EXISTS deprecated,
  DROP COLUMN IF EXISTS successor_subject,
  DROP COLUMN IF EXISTS deprecation_note;
-- +goose StatementEnd
