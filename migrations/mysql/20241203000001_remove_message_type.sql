-- +goose Up
DROP INDEX idx_message_templates_type_status ON message_templates;
ALTER TABLE message_templates DROP COLUMN message_type;
CREATE INDEX idx_message_templates_status ON message_templates(status);

-- +goose Down
DROP INDEX idx_message_templates_status ON message_templates;
ALTER TABLE message_templates ADD COLUMN message_type VARCHAR(20) NOT NULL DEFAULT 'sms';
CREATE INDEX idx_message_templates_type_status ON message_templates(message_type, status);
