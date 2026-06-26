-- +goose Up
ALTER TABLE channel_template_bindings DROP COLUMN IF EXISTS template_binding_id;

-- +goose Down
ALTER TABLE channel_template_bindings ADD COLUMN template_binding_id BIGINT;
