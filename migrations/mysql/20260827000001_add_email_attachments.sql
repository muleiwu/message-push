-- +goose Up
CREATE TABLE email_attachments (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    attachment_group_id VARCHAR(36) NOT NULL,
    position INT NOT NULL,
    filename VARCHAR(255) NOT NULL,
    content_type VARCHAR(255) NOT NULL,
    size_bytes BIGINT UNSIGNED NOT NULL,
    sha256 CHAR(64) NOT NULL,
    content LONGBLOB NULL,
    purged_at TIMESTAMP NULL,
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_email_attachments_group_position (attachment_group_id, position),
    KEY idx_email_attachments_group (attachment_group_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE push_tasks ADD COLUMN attachment_group_id VARCHAR(36) NULL;
CREATE INDEX idx_push_tasks_attachment_group ON push_tasks(attachment_group_id);

-- +goose Down
DROP INDEX idx_push_tasks_attachment_group ON push_tasks;
ALTER TABLE push_tasks DROP COLUMN attachment_group_id;
DROP TABLE email_attachments;
