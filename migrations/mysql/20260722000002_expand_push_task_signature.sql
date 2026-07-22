-- +goose Up
ALTER TABLE push_tasks MODIFY COLUMN signature VARCHAR(200);

-- +goose Down
ALTER TABLE push_tasks MODIFY COLUMN signature VARCHAR(50);
