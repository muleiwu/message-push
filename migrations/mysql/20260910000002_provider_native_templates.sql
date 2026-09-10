-- +goose Up
-- Identity conflicts stop migration; no existing records are merged or deleted.
ALTER TABLE provider_templates ADD COLUMN active_template_code VARCHAR(100) GENERATED ALWAYS AS (CASE WHEN deleted_at IS NULL THEN template_code ELSE NULL END) STORED;
CREATE UNIQUE INDEX uk_provider_templates_account_code_live ON provider_templates (provider_id, active_template_code);
ALTER TABLE provider_templates ADD COLUMN content_version BIGINT NOT NULL DEFAULT 1;
ALTER TABLE channel_template_bindings ADD COLUMN mapped_content_version BIGINT NOT NULL DEFAULT 0;
UPDATE provider_templates SET template_content = native_content WHERE provider_id IN (SELECT id FROM provider_accounts WHERE provider_type = 'sms') AND native_content IS NOT NULL AND native_content <> '';
UPDATE provider_templates SET variables = '[]', content_type = 'text' WHERE provider_id IN (SELECT id FROM provider_accounts WHERE provider_type = 'sms');
UPDATE channel_template_bindings SET param_mapping = '[]', mapped_content_version = 0 WHERE channel_id IN (SELECT id FROM channels WHERE type = 'sms') OR provider_template_id IN (SELECT id FROM provider_templates WHERE provider_id IN (SELECT id FROM provider_accounts WHERE provider_type = 'sms'));
DROP INDEX idx_provider_templates_remote_resource ON provider_templates;
ALTER TABLE provider_templates DROP COLUMN remote_id;
ALTER TABLE provider_templates DROP COLUMN native_content;
ALTER TABLE provider_templates DROP COLUMN variable_slots;
ALTER TABLE provider_templates DROP COLUMN codec_version;

-- +goose Down
-- Old mappings cannot be reconstructed. Disable SMS bindings before older code
-- can interpret an empty mapping as implicit same-name delivery.
UPDATE channel_template_bindings SET status = 0 WHERE channel_id IN (SELECT id FROM channels WHERE type = 'sms') OR provider_template_id IN (SELECT id FROM provider_templates WHERE provider_id IN (SELECT id FROM provider_accounts WHERE provider_type = 'sms'));
ALTER TABLE provider_templates ADD COLUMN remote_id VARCHAR(100) NOT NULL DEFAULT '';
ALTER TABLE provider_templates ADD COLUMN native_content TEXT;
ALTER TABLE provider_templates ADD COLUMN variable_slots TEXT;
ALTER TABLE provider_templates ADD COLUMN codec_version VARCHAR(100) NOT NULL DEFAULT '';
UPDATE provider_templates SET remote_id = template_code, native_content = template_content WHERE synced_at IS NOT NULL;
CREATE INDEX idx_provider_templates_remote_resource ON provider_templates (provider_id, remote_id);
ALTER TABLE channel_template_bindings DROP COLUMN mapped_content_version;
ALTER TABLE provider_templates DROP COLUMN content_version;
DROP INDEX uk_provider_templates_account_code_live ON provider_templates;
ALTER TABLE provider_templates DROP COLUMN active_template_code;
