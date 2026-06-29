-- +goose Up
DROP INDEX IF EXISTS uk_message_templates_template_code;
ALTER TABLE message_templates DROP COLUMN IF EXISTS template_code;

-- +goose Down
ALTER TABLE message_templates ADD COLUMN template_code VARCHAR(100);
CREATE UNIQUE INDEX uk_message_templates_template_code ON message_templates(template_code);
