-- +goose Up
ALTER TABLE push_logs ADD COLUMN provider_msg_id VARCHAR(100);
CREATE INDEX idx_push_logs_provider_msg_id ON push_logs(provider_msg_id);
-- 历史数据迁移（push_tasks.provider_msg_id -> 最新 push_log）
UPDATE push_logs pl
SET provider_msg_id = pt.provider_msg_id
FROM push_tasks pt
WHERE pl.task_id = pt.task_id
  AND pt.provider_msg_id IS NOT NULL AND pt.provider_msg_id <> ''
  AND (pl.provider_msg_id IS NULL OR pl.provider_msg_id = '')
  AND pl.id = (SELECT MAX(id) FROM push_logs x WHERE x.task_id = pt.task_id);
DROP INDEX IF EXISTS idx_push_tasks_provider_msg_id;
ALTER TABLE push_tasks DROP COLUMN IF EXISTS provider_msg_id;

-- +goose Down
ALTER TABLE push_tasks ADD COLUMN provider_msg_id VARCHAR(100);
CREATE INDEX idx_push_tasks_provider_msg_id ON push_tasks(provider_msg_id);
DROP INDEX IF EXISTS idx_push_logs_provider_msg_id;
ALTER TABLE push_logs DROP COLUMN IF EXISTS provider_msg_id;
