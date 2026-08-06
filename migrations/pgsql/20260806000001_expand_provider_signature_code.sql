-- +goose Up
ALTER TABLE provider_signatures ALTER COLUMN signature_code TYPE VARCHAR(200);

-- +goose Down
ALTER TABLE provider_signatures ALTER COLUMN signature_code TYPE VARCHAR(100);
