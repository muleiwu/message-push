-- +goose Up
ALTER TABLE push_tasks DROP COLUMN title;

-- +goose Down
ALTER TABLE push_tasks ADD COLUMN title VARCHAR(200);
