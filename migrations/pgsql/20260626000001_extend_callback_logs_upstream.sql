-- +goose Up
ALTER TABLE callback_logs ADD COLUMN type VARCHAR(20) NOT NULL DEFAULT 'report';
ALTER TABLE callback_logs ADD COLUMN mobile VARCHAR(20);
ALTER TABLE callback_logs ADD COLUMN content TEXT;
CREATE INDEX idx_callback_logs_type ON callback_logs(type);
CREATE INDEX idx_callback_logs_mobile ON callback_logs(mobile);

-- +goose Down
DROP INDEX IF EXISTS idx_callback_logs_type;
DROP INDEX IF EXISTS idx_callback_logs_mobile;
ALTER TABLE callback_logs DROP COLUMN IF EXISTS content;
ALTER TABLE callback_logs DROP COLUMN IF EXISTS mobile;
ALTER TABLE callback_logs DROP COLUMN IF EXISTS type;
