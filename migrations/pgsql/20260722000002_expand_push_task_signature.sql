-- +goose Up
ALTER TABLE push_tasks ALTER COLUMN signature TYPE VARCHAR(200);

-- +goose Down
ALTER TABLE push_tasks ALTER COLUMN signature TYPE VARCHAR(50);
