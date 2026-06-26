-- +goose Up
ALTER TABLE push_tasks DROP COLUMN content;

-- +goose Down
ALTER TABLE push_tasks ADD COLUMN content TEXT;
