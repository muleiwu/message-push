-- +goose Up
-- MySQL TIMESTAMP values are already stored internally as UTC. Runtime
-- connections pin both the driver location and session time_zone to UTC.
SELECT 1;

-- +goose Down
SELECT 1;
