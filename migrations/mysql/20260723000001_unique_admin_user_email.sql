-- +goose Up
UPDATE admin_users
SET email = LOWER(TRIM(email))
WHERE email IS NOT NULL AND TRIM(email) <> '';

UPDATE admin_users
SET email = NULL
WHERE email IS NOT NULL AND TRIM(email) = '';

-- 先以临时名创建唯一索引。若历史数据在规范化后重复，
-- CREATE 会直接失败，并保留原普通索引供人工处理。
CREATE UNIQUE INDEX uk_admin_users_email_migration ON admin_users(email);
DROP INDEX idx_admin_users_email ON admin_users;
ALTER TABLE admin_users RENAME INDEX uk_admin_users_email_migration TO idx_admin_users_email;

UPDATE admin_users SET status = 0 WHERE status = 2;

-- +goose Down
DROP INDEX idx_admin_users_email ON admin_users;
CREATE INDEX idx_admin_users_email ON admin_users(email);
