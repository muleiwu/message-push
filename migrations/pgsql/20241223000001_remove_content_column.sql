-- +goose Up
ALTER TABLE push_tasks DROP COLUMN IF EXISTS content;

-- +goose Down
ALTER TABLE push_tasks ADD COLUMN content TEXT;
