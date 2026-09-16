-- +goose Up
ALTER TABLE idempotency_keys
    DROP COLUMN status_code,
    DROP COLUMN response_body;

-- +goose Down
ALTER TABLE idempotency_keys
    ADD COLUMN status_code int NOT NULL DEFAULT 0,
    ADD COLUMN response_body jsonb NOT NULL DEFAULT '{}'::jsonb;
