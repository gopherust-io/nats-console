-- +goose Up
-- +goose StatementBegin
-- Credential encryption is applied at the application layer (AES-GCM).
-- This migration records the schema version bump; plaintext tokens are
-- re-encrypted on startup when ENCRYPTION_KEY is configured.
SELECT 1;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
