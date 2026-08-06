-- +goose Up
-- SQLite does not enforce the declared VARCHAR length, so existing databases
-- already accept 200-character mapped values without rebuilding the table.

-- +goose Down
-- No-op for the same reason.
