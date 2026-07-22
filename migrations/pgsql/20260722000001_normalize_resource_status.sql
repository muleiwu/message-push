-- +goose Up
UPDATE applications SET status = 0 WHERE status = 2;
UPDATE provider_accounts SET status = 0 WHERE status = 2;
UPDATE channels SET status = 0 WHERE status = 2;
UPDATE channel_template_bindings SET status = 0 WHERE status = 2;

-- +goose Down
-- This data normalization is intentionally irreversible. Converting every
-- disabled row back to the legacy value would also change rows created after
-- this migration.
