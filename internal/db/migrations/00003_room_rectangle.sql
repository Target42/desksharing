-- +goose Up
ALTER TABLE rooms
    ADD COLUMN IF NOT EXISTS rectangle JSONB;

-- +goose Down
ALTER TABLE rooms
    DROP COLUMN IF EXISTS rectangle;
