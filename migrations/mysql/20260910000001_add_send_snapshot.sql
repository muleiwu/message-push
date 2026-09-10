-- +goose Up
ALTER TABLE push_logs ADD COLUMN send_snapshot JSON NULL COMMENT '发送时内容与参数映射快照';

-- +goose Down
ALTER TABLE push_logs DROP COLUMN send_snapshot;
