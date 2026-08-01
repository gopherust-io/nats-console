-- +goose Up
-- +goose StatementBegin
-- Drop JWT resolver account imports (feature removed).

DROP TABLE IF EXISTS nats_jwt_accounts;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
