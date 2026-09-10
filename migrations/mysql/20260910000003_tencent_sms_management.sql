-- +goose Up
ALTER TABLE provider_templates ADD COLUMN provider_metadata JSON NULL;
ALTER TABLE provider_signatures ADD COLUMN provider_metadata JSON NULL;
ALTER TABLE callback_logs ADD COLUMN provider_account_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE callback_logs ADD COLUMN source VARCHAR(20) NOT NULL DEFAULT '';
ALTER TABLE callback_logs ADD COLUMN sms_event_key VARCHAR(64) NULL;
ALTER TABLE callback_logs ADD COLUMN attribution VARCHAR(30) NOT NULL DEFAULT '';
CREATE INDEX idx_callback_logs_account ON callback_logs(provider_account_id);
CREATE UNIQUE INDEX uk_callback_logs_sms_event ON callback_logs(sms_event_key);

CREATE TABLE provider_sms_events (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
 event_key VARCHAR(64) NOT NULL,
 provider_account_id BIGINT NOT NULL,
 sdk_app_id VARCHAR(100) NOT NULL,
 kind VARCHAR(20) NOT NULL,
 source VARCHAR(20) NOT NULL,
 provider_msg_id VARCHAR(100),
 mobile VARCHAR(30),
 status VARCHAR(30),
 error_code VARCHAR(200),
 error_message TEXT,
 occurred_at DATETIME(6) NULL,
 content TEXT,
 sign_name VARCHAR(200),
 extend_code VARCHAR(100),
 raw_data TEXT,
 request_id VARCHAR(100),
 state VARCHAR(20) NOT NULL,
 attempts INTEGER NOT NULL DEFAULT 0,
 next_attempt_at DATETIME(6) NULL,
 last_error TEXT,
 app_id VARCHAR(32),
 attribution VARCHAR(30),
 effect TEXT,
 processed_at DATETIME(6) NULL,
 created_at DATETIME(6) NOT NULL,
 updated_at DATETIME(6) NOT NULL
);
CREATE UNIQUE INDEX uk_provider_sms_events_key ON provider_sms_events(event_key);
CREATE INDEX idx_provider_sms_events_account ON provider_sms_events(provider_account_id, created_at);
CREATE INDEX idx_provider_sms_events_dispatch ON provider_sms_events(state, next_attempt_at);

CREATE TABLE provider_sms_pull_runs (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
 provider_account_id BIGINT NOT NULL,
 sdk_app_id VARCHAR(100) NOT NULL,
 kind VARCHAR(20) NOT NULL,
 source VARCHAR(20) NOT NULL,
 state VARCHAR(20) NOT NULL,
 request_id VARCHAR(100),
 received INTEGER NOT NULL DEFAULT 0,
 inserted INTEGER NOT NULL DEFAULT 0,
 duplicates INTEGER NOT NULL DEFAULT 0,
 error_code VARCHAR(200),
 error_message TEXT,
 started_at DATETIME(6) NOT NULL,
 finished_at DATETIME(6) NULL
);
CREATE INDEX idx_provider_sms_pull_runs_account ON provider_sms_pull_runs(provider_account_id, started_at);
CREATE INDEX idx_provider_sms_pull_runs_state ON provider_sms_pull_runs(state, started_at);

CREATE TABLE provider_sms_polling (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
 provider_account_id BIGINT NOT NULL,
 kind VARCHAR(20) NOT NULL,
 sdk_app_id VARCHAR(100) NOT NULL,
 owner_key VARCHAR(150) NULL,
 enabled BOOLEAN NOT NULL DEFAULT false,
 interval_seconds INTEGER NOT NULL DEFAULT 30,
 suspended BOOLEAN NOT NULL DEFAULT false,
 suspend_reason TEXT,
 next_poll_at DATETIME(6) NULL,
 failures INTEGER NOT NULL DEFAULT 0,
 last_success_at DATETIME(6) NULL,
 last_run_id BIGINT NOT NULL DEFAULT 0,
 last_error TEXT,
 updated_at DATETIME(6) NOT NULL
);
CREATE UNIQUE INDEX uk_provider_sms_polling_account_kind ON provider_sms_polling(provider_account_id, kind);
CREATE UNIQUE INDEX uk_provider_sms_polling_owner ON provider_sms_polling(owner_key);
CREATE INDEX idx_provider_sms_polling_due ON provider_sms_polling(enabled, suspended, next_poll_at);

-- +goose Down
DROP TABLE provider_sms_polling;
DROP TABLE provider_sms_pull_runs;
DROP TABLE provider_sms_events;
DROP INDEX uk_callback_logs_sms_event ON callback_logs;
DROP INDEX idx_callback_logs_account ON callback_logs;
ALTER TABLE callback_logs DROP COLUMN attribution;
ALTER TABLE callback_logs DROP COLUMN sms_event_key;
ALTER TABLE callback_logs DROP COLUMN source;
ALTER TABLE callback_logs DROP COLUMN provider_account_id;
ALTER TABLE provider_signatures DROP COLUMN provider_metadata;
ALTER TABLE provider_templates DROP COLUMN provider_metadata;
