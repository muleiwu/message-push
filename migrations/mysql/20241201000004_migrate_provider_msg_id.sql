-- +goose Up
ALTER TABLE push_logs ADD COLUMN provider_msg_id VARCHAR(100);
CREATE INDEX idx_push_logs_provider_msg_id ON push_logs(provider_msg_id);
-- 历史数据迁移（push_tasks.provider_msg_id -> 最新 push_log）
UPDATE push_logs pl
INNER JOIN (
    SELECT task_id, provider_msg_id FROM push_tasks
    WHERE provider_msg_id IS NOT NULL AND provider_msg_id <> ''
) pt ON pl.task_id = pt.task_id
INNER JOIN (
    SELECT task_id, MAX(id) AS max_id FROM push_logs GROUP BY task_id
) latest ON pl.task_id = latest.task_id AND pl.id = latest.max_id
SET pl.provider_msg_id = pt.provider_msg_id
WHERE pl.provider_msg_id IS NULL OR pl.provider_msg_id = '';
DROP INDEX idx_push_tasks_provider_msg_id ON push_tasks;
ALTER TABLE push_tasks DROP COLUMN provider_msg_id;

-- +goose Down
ALTER TABLE push_tasks ADD COLUMN provider_msg_id VARCHAR(100);
CREATE INDEX idx_push_tasks_provider_msg_id ON push_tasks(provider_msg_id);
DROP INDEX idx_push_logs_provider_msg_id ON push_logs;
ALTER TABLE push_logs DROP COLUMN provider_msg_id;
