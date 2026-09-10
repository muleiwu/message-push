-- +goose Up
ALTER TABLE push_logs ADD COLUMN send_snapshot TEXT;

-- +goose Down
ALTER TABLE push_logs DROP COLUMN send_snapshot;
