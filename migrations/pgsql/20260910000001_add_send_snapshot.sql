-- +goose Up
ALTER TABLE push_logs ADD COLUMN send_snapshot JSONB;
COMMENT ON COLUMN push_logs.send_snapshot IS '发送时内容与参数映射快照';

-- +goose Down
ALTER TABLE push_logs DROP COLUMN send_snapshot;
