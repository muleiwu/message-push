-- +goose Up
ALTER TABLE provider_signatures MODIFY COLUMN signature_code VARCHAR(200) NOT NULL;

-- +goose Down
ALTER TABLE provider_signatures MODIFY COLUMN signature_code VARCHAR(100) NOT NULL;
