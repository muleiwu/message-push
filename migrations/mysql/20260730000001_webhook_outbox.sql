-- +goose Up
ALTER TABLE webhook_logs
    ADD COLUMN dedup_key VARCHAR(128) NULL,
    ADD COLUMN signing_secret VARCHAR(64) NULL,
    ADD COLUMN max_retries INT NOT NULL DEFAULT 3,
    ADD COLUMN timeout_seconds INT NOT NULL DEFAULT 5,
    ADD COLUMN next_attempt_at TIMESTAMP NULL,
    ADD COLUMN locked_until TIMESTAMP NULL,
    ADD COLUMN lease_token VARCHAR(64) NULL,
    ADD COLUMN updated_at TIMESTAMP NULL;

UPDATE webhook_logs
SET updated_at = created_at
WHERE updated_at IS NULL;

CREATE UNIQUE INDEX uk_webhook_logs_dedup_key ON webhook_logs(dedup_key);
CREATE INDEX idx_webhook_logs_dispatch ON webhook_logs(status, next_attempt_at);

-- +goose Down
DROP INDEX idx_webhook_logs_dispatch ON webhook_logs;
DROP INDEX uk_webhook_logs_dedup_key ON webhook_logs;

ALTER TABLE webhook_logs
    DROP COLUMN updated_at,
    DROP COLUMN lease_token,
    DROP COLUMN locked_until,
    DROP COLUMN next_attempt_at,
    DROP COLUMN timeout_seconds,
    DROP COLUMN max_retries,
    DROP COLUMN signing_secret,
    DROP COLUMN dedup_key;
