-- +goose Up
UPDATE admin_users
SET email = LOWER(TRIM(email))
WHERE email IS NOT NULL AND TRIM(email) <> '';

UPDATE admin_users
SET email = NULL
WHERE email IS NOT NULL AND TRIM(email) = '';

DROP INDEX IF EXISTS idx_admin_users_email;
CREATE UNIQUE INDEX idx_admin_users_email ON admin_users(email);

UPDATE admin_users SET status = 0 WHERE status = 2;

-- +goose Down
DROP INDEX IF EXISTS idx_admin_users_email;
CREATE INDEX idx_admin_users_email ON admin_users(email);
