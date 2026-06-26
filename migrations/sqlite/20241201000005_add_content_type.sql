-- +goose Up
ALTER TABLE message_templates ADD COLUMN content_type VARCHAR(20) DEFAULT 'text';
ALTER TABLE provider_templates ADD COLUMN content_type VARCHAR(20) DEFAULT 'text';

-- +goose Down
ALTER TABLE message_templates DROP COLUMN content_type;
ALTER TABLE provider_templates DROP COLUMN content_type;
