-- +goose Up
ALTER TABLE provider_templates ADD COLUMN remote_id VARCHAR(100) NOT NULL DEFAULT '';
ALTER TABLE provider_templates ADD COLUMN audit_status SMALLINT NULL;
ALTER TABLE provider_templates ADD COLUMN audit_reply TEXT;
ALTER TABLE provider_templates ADD COLUMN remote_deleted BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE provider_templates ADD COLUMN synced_at DATETIME NULL;
ALTER TABLE provider_templates ADD COLUMN native_content TEXT;
ALTER TABLE provider_templates ADD COLUMN variable_slots TEXT;
ALTER TABLE provider_templates ADD COLUMN codec_version VARCHAR(100) NOT NULL DEFAULT '';
ALTER TABLE provider_templates ADD COLUMN remote_name VARCHAR(200) NOT NULL DEFAULT '';
ALTER TABLE provider_templates ADD COLUMN category VARCHAR(20) NOT NULL DEFAULT '';
ALTER TABLE provider_templates ADD COLUMN remote_description TEXT;
CREATE INDEX idx_provider_templates_remote_resource ON provider_templates (provider_id, remote_id);
ALTER TABLE provider_signatures ADD COLUMN remote_id VARCHAR(100) NOT NULL DEFAULT '';
ALTER TABLE provider_signatures ADD COLUMN audit_status SMALLINT NULL;
ALTER TABLE provider_signatures ADD COLUMN audit_reply TEXT;
ALTER TABLE provider_signatures ADD COLUMN remote_deleted BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE provider_signatures ADD COLUMN synced_at DATETIME NULL;
ALTER TABLE provider_signatures ADD COLUMN remote_description TEXT;
CREATE INDEX idx_provider_signatures_remote_resource ON provider_signatures (provider_account_id, remote_id);

-- +goose Down
-- Keep unavailable remote resources disabled when older code loses audit metadata.
UPDATE provider_templates SET status = 0 WHERE remote_deleted = TRUE OR (audit_status IS NOT NULL AND audit_status <> 2);
UPDATE provider_signatures SET status = 0 WHERE remote_deleted = TRUE OR (audit_status IS NOT NULL AND audit_status <> 2);
DROP INDEX idx_provider_templates_remote_resource;
ALTER TABLE provider_templates DROP COLUMN remote_description;
ALTER TABLE provider_templates DROP COLUMN category;
ALTER TABLE provider_templates DROP COLUMN remote_name;
ALTER TABLE provider_templates DROP COLUMN codec_version;
ALTER TABLE provider_templates DROP COLUMN variable_slots;
ALTER TABLE provider_templates DROP COLUMN native_content;
ALTER TABLE provider_templates DROP COLUMN synced_at;
ALTER TABLE provider_templates DROP COLUMN remote_deleted;
ALTER TABLE provider_templates DROP COLUMN audit_reply;
ALTER TABLE provider_templates DROP COLUMN audit_status;
ALTER TABLE provider_templates DROP COLUMN remote_id;
DROP INDEX idx_provider_signatures_remote_resource;
ALTER TABLE provider_signatures DROP COLUMN remote_description;
ALTER TABLE provider_signatures DROP COLUMN synced_at;
ALTER TABLE provider_signatures DROP COLUMN remote_deleted;
ALTER TABLE provider_signatures DROP COLUMN audit_reply;
ALTER TABLE provider_signatures DROP COLUMN audit_status;
ALTER TABLE provider_signatures DROP COLUMN remote_id;
