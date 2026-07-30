-- +goose Up
ALTER TABLE webhook_logs ADD COLUMN dedup_key VARCHAR(128);
ALTER TABLE webhook_logs ADD COLUMN signing_secret VARCHAR(64);
ALTER TABLE webhook_logs ADD COLUMN max_retries INTEGER NOT NULL DEFAULT 3;
ALTER TABLE webhook_logs ADD COLUMN timeout_seconds INTEGER NOT NULL DEFAULT 5;
ALTER TABLE webhook_logs ADD COLUMN next_attempt_at DATETIME;
ALTER TABLE webhook_logs ADD COLUMN locked_until DATETIME;
ALTER TABLE webhook_logs ADD COLUMN lease_token VARCHAR(64);
ALTER TABLE webhook_logs ADD COLUMN updated_at DATETIME;

UPDATE webhook_logs
SET updated_at = created_at
WHERE updated_at IS NULL;

CREATE UNIQUE INDEX uk_webhook_logs_dedup_key ON webhook_logs(dedup_key);
CREATE INDEX idx_webhook_logs_dispatch ON webhook_logs(status, next_attempt_at);

-- +goose Down
DROP INDEX IF EXISTS idx_webhook_logs_dispatch;
DROP INDEX IF EXISTS uk_webhook_logs_dedup_key;

ALTER TABLE webhook_logs DROP COLUMN updated_at;
ALTER TABLE webhook_logs DROP COLUMN lease_token;
ALTER TABLE webhook_logs DROP COLUMN locked_until;
ALTER TABLE webhook_logs DROP COLUMN next_attempt_at;
ALTER TABLE webhook_logs DROP COLUMN timeout_seconds;
ALTER TABLE webhook_logs DROP COLUMN max_retries;
ALTER TABLE webhook_logs DROP COLUMN signing_secret;
ALTER TABLE webhook_logs DROP COLUMN dedup_key;
