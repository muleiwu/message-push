-- +goose Up
-- +goose StatementBegin
-- 初始 schema（PostgreSQL）。本文件还原最初 AutoMigrate 时的表结构，
-- 后续 0002~0009 迁移会逐条演进到当前模型最终状态。

CREATE TABLE IF NOT EXISTS applications (
    id BIGSERIAL PRIMARY KEY,
    app_id VARCHAR(32) NOT NULL,
    app_secret VARCHAR(128) NOT NULL,
    app_name VARCHAR(100) NOT NULL,
    status SMALLINT DEFAULT 1,
    ip_whitelist TEXT,
    webhook_url VARCHAR(255),
    daily_quota INTEGER DEFAULT 10000,
    rate_limit INTEGER DEFAULT 100,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_applications_app_id ON applications(app_id);
CREATE INDEX IF NOT EXISTS idx_applications_status ON applications(status);
CREATE INDEX IF NOT EXISTS idx_applications_deleted_at ON applications(deleted_at);

CREATE TABLE IF NOT EXISTS provider_accounts (
    id BIGSERIAL PRIMARY KEY,
    account_code VARCHAR(50) NOT NULL,
    account_name VARCHAR(100) NOT NULL,
    provider_code VARCHAR(50) NOT NULL,
    provider_type VARCHAR(20) NOT NULL,
    config JSONB NOT NULL,
    status SMALLINT DEFAULT 1,
    remark TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_provider_accounts_account_code ON provider_accounts(account_code);
CREATE INDEX IF NOT EXISTS idx_provider_accounts_provider_type ON provider_accounts(provider_code, provider_type);
CREATE INDEX IF NOT EXISTS idx_provider_accounts_status ON provider_accounts(status);
CREATE INDEX IF NOT EXISTS idx_provider_accounts_deleted_at ON provider_accounts(deleted_at);

CREATE TABLE IF NOT EXISTS provider_signatures (
    id BIGSERIAL PRIMARY KEY,
    provider_account_id BIGINT NOT NULL,
    signature_code VARCHAR(100) NOT NULL,
    signature_name VARCHAR(100) NOT NULL,
    status SMALLINT DEFAULT 1,
    remark TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_provider_signatures_provider_account ON provider_signatures(provider_account_id);
CREATE INDEX IF NOT EXISTS idx_provider_signatures_status ON provider_signatures(status);
CREATE INDEX IF NOT EXISTS idx_provider_signatures_deleted_at ON provider_signatures(deleted_at);

CREATE TABLE IF NOT EXISTS channels (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(20) NOT NULL,
    message_template_id BIGINT,
    status SMALLINT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_channels_type ON channels(type);
CREATE INDEX IF NOT EXISTS idx_channels_message_template ON channels(message_template_id);
CREATE INDEX IF NOT EXISTS idx_channels_status ON channels(status);
CREATE INDEX IF NOT EXISTS idx_channels_deleted_at ON channels(deleted_at);

CREATE TABLE IF NOT EXISTS push_tasks (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(36) NOT NULL,
    app_id VARCHAR(32) NOT NULL,
    channel_id BIGINT NOT NULL,
    message_type VARCHAR(20) NOT NULL,
    receiver VARCHAR(100) NOT NULL,
    content TEXT,
    title VARCHAR(200),
    template_code VARCHAR(50),
    template_params JSONB,
    signature VARCHAR(50),
    status VARCHAR(20) DEFAULT 'pending',
    callback_status VARCHAR(20),
    callback_time TIMESTAMP,
    retry_count INTEGER DEFAULT 0,
    max_retry INTEGER DEFAULT 3,
    provider_msg_id VARCHAR(100),
    exclude_provider_ids JSONB,
    scheduled_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_push_tasks_task_id ON push_tasks(task_id);
CREATE INDEX IF NOT EXISTS idx_push_tasks_app_id_status ON push_tasks(app_id, status);
CREATE INDEX IF NOT EXISTS idx_push_tasks_channel ON push_tasks(channel_id);
CREATE INDEX IF NOT EXISTS idx_push_tasks_status_scheduled ON push_tasks(status, scheduled_at);
CREATE INDEX IF NOT EXISTS idx_push_tasks_provider_msg_id ON push_tasks(provider_msg_id);
CREATE INDEX IF NOT EXISTS idx_push_tasks_created_at ON push_tasks(created_at);

CREATE TABLE IF NOT EXISTS push_batch_tasks (
    id BIGSERIAL PRIMARY KEY,
    batch_id VARCHAR(36) NOT NULL,
    app_id VARCHAR(32) NOT NULL,
    total_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    failed_count INTEGER DEFAULT 0,
    pending_count INTEGER DEFAULT 0,
    status VARCHAR(20) DEFAULT 'processing',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_push_batch_tasks_batch_id ON push_batch_tasks(batch_id);
CREATE INDEX IF NOT EXISTS idx_push_batch_tasks_app_id_status ON push_batch_tasks(app_id, status);
CREATE INDEX IF NOT EXISTS idx_push_batch_tasks_created_at ON push_batch_tasks(created_at);

CREATE TABLE IF NOT EXISTS push_logs (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(36) NOT NULL,
    app_id VARCHAR(32) NOT NULL,
    provider_account_id BIGINT NOT NULL,
    request_data JSONB,
    response_data JSONB,
    status VARCHAR(20) NOT NULL,
    error_message TEXT,
    cost_time INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_push_logs_task_id ON push_logs(task_id);
CREATE INDEX IF NOT EXISTS idx_push_logs_app_id_created ON push_logs(app_id, created_at);
CREATE INDEX IF NOT EXISTS idx_push_logs_provider_account ON push_logs(provider_account_id);
CREATE INDEX IF NOT EXISTS idx_push_logs_status_created ON push_logs(status, created_at);
CREATE INDEX IF NOT EXISTS idx_push_logs_created_at ON push_logs(created_at);

CREATE TABLE IF NOT EXISTS message_templates (
    id BIGSERIAL PRIMARY KEY,
    template_name VARCHAR(200) NOT NULL,
    template_code VARCHAR(100),
    message_type VARCHAR(20) NOT NULL DEFAULT 'sms',
    content TEXT NOT NULL,
    variables JSONB,
    description TEXT,
    status SMALLINT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_message_templates_template_code ON message_templates(template_code);
CREATE INDEX IF NOT EXISTS idx_message_templates_type_status ON message_templates(message_type, status);
CREATE INDEX IF NOT EXISTS idx_message_templates_deleted_at ON message_templates(deleted_at);

CREATE TABLE IF NOT EXISTS provider_templates (
    id BIGSERIAL PRIMARY KEY,
    provider_id BIGINT NOT NULL,
    template_code VARCHAR(100) NOT NULL,
    template_name VARCHAR(200) NOT NULL,
    template_content TEXT,
    variables JSONB,
    status SMALLINT DEFAULT 1,
    remark TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_provider_templates_provider_status ON provider_templates(provider_id, status);
CREATE INDEX IF NOT EXISTS idx_provider_templates_deleted_at ON provider_templates(deleted_at);

CREATE TABLE IF NOT EXISTS channel_template_bindings (
    id BIGSERIAL PRIMARY KEY,
    channel_id BIGINT NOT NULL,
    provider_template_id BIGINT NOT NULL,
    provider_id BIGINT NOT NULL,
    template_binding_id BIGINT,
    param_mapping JSONB,
    weight INTEGER DEFAULT 10,
    priority INTEGER DEFAULT 100,
    status SMALLINT DEFAULT 1,
    is_active SMALLINT DEFAULT 1,
    auto_disable_on_fail BOOLEAN DEFAULT FALSE,
    auto_disable_threshold INTEGER DEFAULT 5,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_channel_template_bindings_channel ON channel_template_bindings(channel_id);
CREATE INDEX IF NOT EXISTS idx_channel_template_bindings_provider ON channel_template_bindings(provider_id);
CREATE INDEX IF NOT EXISTS idx_channel_template_bindings_deleted_at ON channel_template_bindings(deleted_at);

CREATE TABLE IF NOT EXISTS channel_signature_mappings (
    id BIGSERIAL PRIMARY KEY,
    channel_id BIGINT NOT NULL,
    signature_name VARCHAR(100) NOT NULL,
    provider_signature_id BIGINT NOT NULL,
    provider_id BIGINT NOT NULL,
    status SMALLINT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_channel_signature_mappings_channel ON channel_signature_mappings(channel_id);
CREATE INDEX IF NOT EXISTS idx_channel_signature_mappings_provider ON channel_signature_mappings(provider_id);
CREATE INDEX IF NOT EXISTS idx_channel_signature_mappings_deleted_at ON channel_signature_mappings(deleted_at);

CREATE TABLE IF NOT EXISTS channel_health_history (
    id BIGSERIAL PRIMARY KEY,
    provider_channel_id BIGINT NOT NULL,
    check_time TIMESTAMP NOT NULL,
    status VARCHAR(20) NOT NULL,
    response_time INTEGER,
    error_count INTEGER DEFAULT 0,
    success_rate DECIMAL(5,2),
    is_available SMALLINT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_channel_health_history_channel_time ON channel_health_history(provider_channel_id, check_time);
CREATE INDEX IF NOT EXISTS idx_channel_health_history_check_time ON channel_health_history(check_time);

CREATE TABLE IF NOT EXISTS app_quota_stats (
    id BIGSERIAL PRIMARY KEY,
    app_id VARCHAR(32) NOT NULL,
    stat_date DATE NOT NULL,
    total_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    failed_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_app_quota_stats_app_date ON app_quota_stats(app_id, stat_date);
CREATE INDEX IF NOT EXISTS idx_app_quota_stats_stat_date ON app_quota_stats(stat_date);

CREATE TABLE IF NOT EXISTS provider_quota_stats (
    id BIGSERIAL PRIMARY KEY,
    provider_channel_id BIGINT NOT NULL,
    stat_date DATE NOT NULL,
    total_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    failed_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_provider_quota_stats_channel_date ON provider_quota_stats(provider_channel_id, stat_date);
CREATE INDEX IF NOT EXISTS idx_provider_quota_stats_stat_date ON provider_quota_stats(stat_date);

CREATE TABLE IF NOT EXISTS admin_users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL,
    password VARCHAR(255) NOT NULL,
    real_name VARCHAR(100) NOT NULL,
    status SMALLINT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_admin_users_username ON admin_users(username);
CREATE INDEX IF NOT EXISTS idx_admin_users_status ON admin_users(status);
CREATE INDEX IF NOT EXISTS idx_admin_users_deleted_at ON admin_users(deleted_at);

CREATE TABLE IF NOT EXISTS webhook_configs (
    id BIGSERIAL PRIMARY KEY,
    app_id VARCHAR(32) NOT NULL,
    webhook_url VARCHAR(500) NOT NULL,
    secret VARCHAR(64),
    events VARCHAR(200) DEFAULT 'success,failed',
    status INTEGER DEFAULT 1,
    retry_count INTEGER DEFAULT 3,
    timeout INTEGER DEFAULT 5,
    description VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_webhook_configs_app_id ON webhook_configs(app_id);

CREATE TABLE IF NOT EXISTS callback_logs (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(36),
    app_id VARCHAR(32) NOT NULL,
    provider_code VARCHAR(32) NOT NULL,
    provider_id VARCHAR(64),
    callback_status VARCHAR(20),
    error_code VARCHAR(32),
    error_message TEXT,
    raw_data JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_callback_logs_task_id ON callback_logs(task_id);
CREATE INDEX IF NOT EXISTS idx_callback_logs_app_id ON callback_logs(app_id);
CREATE INDEX IF NOT EXISTS idx_callback_logs_provider_code ON callback_logs(provider_code);
CREATE INDEX IF NOT EXISTS idx_callback_logs_provider_id ON callback_logs(provider_id);
CREATE INDEX IF NOT EXISTS idx_callback_logs_created_at ON callback_logs(created_at);

CREATE TABLE IF NOT EXISTS webhook_logs (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(36),
    app_id VARCHAR(32) NOT NULL,
    webhook_config_id BIGINT,
    webhook_url VARCHAR(500) NOT NULL,
    event VARCHAR(20) NOT NULL,
    request_data JSONB,
    response_status INTEGER,
    response_data TEXT,
    status VARCHAR(20) NOT NULL,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_task_id ON webhook_logs(task_id);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_app_id ON webhook_logs(app_id);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_webhook_config ON webhook_logs(webhook_config_id);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_created_at ON webhook_logs(created_at);

CREATE TABLE IF NOT EXISTS failure_rules (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    scene VARCHAR(20) NOT NULL,
    provider_code VARCHAR(50),
    message_type VARCHAR(20),
    error_code VARCHAR(200),
    error_keyword VARCHAR(200),
    action VARCHAR(20) NOT NULL,
    action_config JSONB,
    priority INTEGER DEFAULT 0,
    status SMALLINT DEFAULT 1,
    remark VARCHAR(500),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_failure_rules_scene_priority ON failure_rules(scene, priority);
CREATE INDEX IF NOT EXISTS idx_failure_rules_provider_code ON failure_rules(provider_code);
CREATE INDEX IF NOT EXISTS idx_failure_rules_message_type ON failure_rules(message_type);
CREATE INDEX IF NOT EXISTS idx_failure_rules_status ON failure_rules(status);
CREATE INDEX IF NOT EXISTS idx_failure_rules_deleted_at ON failure_rules(deleted_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS failure_rules;
DROP TABLE IF EXISTS webhook_logs;
DROP TABLE IF EXISTS callback_logs;
DROP TABLE IF EXISTS webhook_configs;
DROP TABLE IF EXISTS admin_users;
DROP TABLE IF EXISTS provider_quota_stats;
DROP TABLE IF EXISTS app_quota_stats;
DROP TABLE IF EXISTS channel_health_history;
DROP TABLE IF EXISTS channel_signature_mappings;
DROP TABLE IF EXISTS channel_template_bindings;
DROP TABLE IF EXISTS provider_templates;
DROP TABLE IF EXISTS message_templates;
DROP TABLE IF EXISTS push_logs;
DROP TABLE IF EXISTS push_batch_tasks;
DROP TABLE IF EXISTS push_tasks;
DROP TABLE IF EXISTS channels;
DROP TABLE IF EXISTS provider_signatures;
DROP TABLE IF EXISTS provider_accounts;
DROP TABLE IF EXISTS applications;
-- +goose StatementEnd
