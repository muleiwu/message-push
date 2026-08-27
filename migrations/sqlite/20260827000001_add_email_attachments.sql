-- +goose Up
CREATE TABLE email_attachments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    attachment_group_id VARCHAR(36) NOT NULL,
    position INTEGER NOT NULL,
    filename VARCHAR(255) NOT NULL,
    content_type VARCHAR(255) NOT NULL,
    size_bytes INTEGER NOT NULL,
    sha256 CHAR(64) NOT NULL,
    content BLOB,
    purged_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX uk_email_attachments_group_position ON email_attachments(attachment_group_id, position);
CREATE INDEX idx_email_attachments_group ON email_attachments(attachment_group_id);

ALTER TABLE push_tasks ADD COLUMN attachment_group_id VARCHAR(36);
CREATE INDEX idx_push_tasks_attachment_group ON push_tasks(attachment_group_id);

-- +goose Down
DROP INDEX IF EXISTS idx_push_tasks_attachment_group;
ALTER TABLE push_tasks DROP COLUMN attachment_group_id;
DROP TABLE email_attachments;
