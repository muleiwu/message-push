-- +goose Up
ALTER TABLE push_tasks
    ADD COLUMN provider_account_id BIGINT UNSIGNED NULL COMMENT '最后一次发送尝试使用的服务商账号ID';

UPDATE push_tasks
SET provider_account_id = (
    SELECT push_logs.provider_account_id
    FROM push_logs
    WHERE push_logs.task_id = push_tasks.task_id
    ORDER BY push_logs.id DESC
    LIMIT 1
)
WHERE EXISTS (
    SELECT 1
    FROM push_logs
    WHERE push_logs.task_id = push_tasks.task_id
);

CREATE INDEX idx_push_tasks_provider_account ON push_tasks(provider_account_id);

-- +goose Down
DROP INDEX idx_push_tasks_provider_account ON push_tasks;
ALTER TABLE push_tasks DROP COLUMN provider_account_id;
