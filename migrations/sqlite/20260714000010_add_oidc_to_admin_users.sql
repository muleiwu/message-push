-- +goose Up
ALTER TABLE admin_users ADD COLUMN email VARCHAR(255) NULL;
ALTER TABLE admin_users ADD COLUMN oidc_sub VARCHAR(255) NULL;
ALTER TABLE admin_users ADD COLUMN auth_source VARCHAR(32) NOT NULL DEFAULT 'local';
CREATE UNIQUE INDEX idx_admin_users_oidc_sub ON admin_users(oidc_sub);
CREATE INDEX idx_admin_users_email ON admin_users(email);

-- +goose Down
DROP INDEX IF EXISTS idx_admin_users_oidc_sub;
DROP INDEX IF EXISTS idx_admin_users_email;
ALTER TABLE admin_users DROP COLUMN auth_source;
ALTER TABLE admin_users DROP COLUMN oidc_sub;
ALTER TABLE admin_users DROP COLUMN email;
