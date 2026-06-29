-- +goose Up
ALTER TABLE push_tasks DROP COLUMN IF EXISTS title;

-- +goose Down
ALTER TABLE push_tasks ADD COLUMN title VARCHAR(200);
