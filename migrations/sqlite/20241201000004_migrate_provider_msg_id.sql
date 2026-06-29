-- +goose Up
ALTER TABLE push_logs ADD COLUMN provider_msg_id VARCHAR(100);
CREATE INDEX idx_push_logs_provider_msg_id ON push_logs(provider_msg_id);
-- 历史数据迁移（push_tasks.provider_msg_id -> 最新 push_log）
UPDATE push_logs
SET provider_msg_id = (
    SELECT pt.provider_msg_id FROM push_tasks pt
    WHERE pt.task_id = push_logs.task_id AND pt.provider_msg_id IS NOT NULL AND pt.provider_msg_id <> ''
)
WHERE id IN (SELECT MAX(id) FROM push_logs GROUP BY task_id)
  AND (provider_msg_id IS NULL OR provider_msg_id = '')
  AND EXISTS (SELECT 1 FROM push_tasks pt WHERE pt.task_id = push_logs.task_id AND pt.provider_msg_id IS NOT NULL AND pt.provider_msg_id <> '');
DROP INDEX IF EXISTS idx_push_tasks_provider_msg_id;
ALTER TABLE push_tasks DROP COLUMN provider_msg_id;

-- +goose Down
ALTER TABLE push_tasks ADD COLUMN provider_msg_id VARCHAR(100);
CREATE INDEX idx_push_tasks_provider_msg_id ON push_tasks(provider_msg_id);
DROP INDEX IF EXISTS idx_push_logs_provider_msg_id;
ALTER TABLE push_logs DROP COLUMN provider_msg_id;
