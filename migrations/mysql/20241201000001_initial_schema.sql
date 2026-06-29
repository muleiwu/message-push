-- +goose Up
-- +goose StatementBegin
-- 初始 schema（MySQL）。本文件还原最初 AutoMigrate 时的表结构，
-- 后续 0002~0009 迁移会逐条演进到当前模型最终状态。

CREATE TABLE applications (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    app_id VARCHAR(32) NOT NULL,
    app_secret VARCHAR(128) NOT NULL,
    app_name VARCHAR(100) NOT NULL,
    status TINYINT DEFAULT 1,
    ip_whitelist TEXT,
    webhook_url VARCHAR(255),
    daily_quota INT DEFAULT 10000,
    rate_limit INT DEFAULT 100,
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    UNIQUE KEY uk_applications_app_id (app_id),
    KEY idx_applications_status (status),
    KEY idx_applications_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE provider_accounts (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    account_code VARCHAR(50) NOT NULL,
    account_name VARCHAR(100) NOT NULL,
    provider_code VARCHAR(50) NOT NULL,
    provider_type VARCHAR(20) NOT NULL,
    config JSON NOT NULL,
    status TINYINT DEFAULT 1,
    remark TEXT,
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    UNIQUE KEY uk_provider_accounts_account_code (account_code),
    KEY idx_provider_accounts_provider_type (provider_code, provider_type),
    KEY idx_provider_accounts_status (status),
    KEY idx_provider_accounts_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE provider_signatures (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    provider_account_id BIGINT UNSIGNED NOT NULL,
    signature_code VARCHAR(100) NOT NULL,
    signature_name VARCHAR(100) NOT NULL,
    status TINYINT DEFAULT 1,
    remark TEXT,
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    KEY idx_provider_signatures_provider_account (provider_account_id),
    KEY idx_provider_signatures_status (status),
    KEY idx_provider_signatures_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE channels (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(20) NOT NULL,
    message_template_id BIGINT UNSIGNED,
    status TINYINT DEFAULT 1,
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    KEY idx_channels_type (type),
    KEY idx_channels_message_template (message_template_id),
    KEY idx_channels_status (status),
    KEY idx_channels_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE push_tasks (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    task_id VARCHAR(36) NOT NULL,
    app_id VARCHAR(32) NOT NULL,
    channel_id BIGINT UNSIGNED NOT NULL,
    message_type VARCHAR(20) NOT NULL,
    receiver VARCHAR(100) NOT NULL,
    content TEXT,
    title VARCHAR(200),
    template_code VARCHAR(50),
    template_params JSON,
    signature VARCHAR(50),
    status VARCHAR(20) DEFAULT 'pending',
    callback_status VARCHAR(20),
    callback_time TIMESTAMP NULL DEFAULT NULL,
    retry_count INT DEFAULT 0,
    max_retry INT DEFAULT 3,
    provider_msg_id VARCHAR(100),
    exclude_provider_ids JSON,
    scheduled_at TIMESTAMP NULL DEFAULT NULL,
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_push_tasks_task_id (task_id),
    KEY idx_push_tasks_app_id_status (app_id, status),
    KEY idx_push_tasks_channel (channel_id),
    KEY idx_push_tasks_status_scheduled (status, scheduled_at),
    KEY idx_push_tasks_provider_msg_id (provider_msg_id),
    KEY idx_push_tasks_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE push_batch_tasks (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    batch_id VARCHAR(36) NOT NULL,
    app_id VARCHAR(32) NOT NULL,
    total_count INT DEFAULT 0,
    success_count INT DEFAULT 0,
    failed_count INT DEFAULT 0,
    pending_count INT DEFAULT 0,
    status VARCHAR(20) DEFAULT 'processing',
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_push_batch_tasks_batch_id (batch_id),
    KEY idx_push_batch_tasks_app_id_status (app_id, status),
    KEY idx_push_batch_tasks_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE push_logs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    task_id VARCHAR(36) NOT NULL,
    app_id VARCHAR(32) NOT NULL,
    provider_account_id BIGINT UNSIGNED NOT NULL,
    request_data JSON,
    response_data JSON,
    status VARCHAR(20) NOT NULL,
    error_message TEXT,
    cost_time INT,
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_push_logs_task_id (task_id),
    KEY idx_push_logs_app_id_created (app_id, created_at),
    KEY idx_push_logs_provider_account (provider_account_id),
    KEY idx_push_logs_status_created (status, created_at),
    KEY idx_push_logs_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE message_templates (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    template_name VARCHAR(200) NOT NULL,
    template_code VARCHAR(100),
    message_type VARCHAR(20) NOT NULL DEFAULT 'sms',
    content TEXT NOT NULL,
    variables JSON,
    description TEXT,
    status TINYINT DEFAULT 1,
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    UNIQUE KEY uk_message_templates_template_code (template_code),
    KEY idx_message_templates_type_status (message_type, status),
    KEY idx_message_templates_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE provider_templates (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    provider_id BIGINT UNSIGNED NOT NULL,
    template_code VARCHAR(100) NOT NULL,
    template_name VARCHAR(200) NOT NULL,
    template_content TEXT,
    variables JSON,
    status TINYINT DEFAULT 1,
    remark TEXT,
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    KEY idx_provider_templates_provider_status (provider_id, status),
    KEY idx_provider_templates_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE channel_template_bindings (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    channel_id BIGINT UNSIGNED NOT NULL,
    provider_template_id BIGINT UNSIGNED NOT NULL,
    provider_id BIGINT UNSIGNED NOT NULL,
    template_binding_id BIGINT UNSIGNED,
    param_mapping JSON,
    weight INT DEFAULT 10,
    priority INT DEFAULT 100,
    status TINYINT DEFAULT 1,
    is_active TINYINT DEFAULT 1,
    auto_disable_on_fail TINYINT(1) DEFAULT 0,
    auto_disable_threshold INT DEFAULT 5,
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    KEY idx_channel_template_bindings_channel (channel_id),
    KEY idx_channel_template_bindings_provider (provider_id),
    KEY idx_channel_template_bindings_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE channel_signature_mappings (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    channel_id BIGINT UNSIGNED NOT NULL,
    signature_name VARCHAR(100) NOT NULL,
    provider_signature_id BIGINT UNSIGNED NOT NULL,
    provider_id BIGINT UNSIGNED NOT NULL,
    status TINYINT DEFAULT 1,
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    KEY idx_channel_signature_mappings_channel (channel_id),
    KEY idx_channel_signature_mappings_provider (provider_id),
    KEY idx_channel_signature_mappings_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE channel_health_history (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    provider_channel_id BIGINT UNSIGNED NOT NULL,
    check_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(20) NOT NULL,
    response_time INT,
    error_count INT DEFAULT 0,
    success_rate DECIMAL(5,2),
    is_available TINYINT DEFAULT 1,
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_channel_health_history_channel_time (provider_channel_id, check_time),
    KEY idx_channel_health_history_check_time (check_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE app_quota_stats (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    app_id VARCHAR(32) NOT NULL,
    stat_date DATE NOT NULL,
    total_count INT DEFAULT 0,
    success_count INT DEFAULT 0,
    failed_count INT DEFAULT 0,
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_app_quota_stats_app_date (app_id, stat_date),
    KEY idx_app_quota_stats_stat_date (stat_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE provider_quota_stats (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    provider_channel_id BIGINT UNSIGNED NOT NULL,
    stat_date DATE NOT NULL,
    total_count INT DEFAULT 0,
    success_count INT DEFAULT 0,
    failed_count INT DEFAULT 0,
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_provider_quota_stats_channel_date (provider_channel_id, stat_date),
    KEY idx_provider_quota_stats_stat_date (stat_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE admin_users (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(50) NOT NULL,
    password VARCHAR(255) NOT NULL,
    real_name VARCHAR(100) NOT NULL,
    status TINYINT DEFAULT 1,
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    UNIQUE KEY uk_admin_users_username (username),
    KEY idx_admin_users_status (status),
    KEY idx_admin_users_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE webhook_configs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    app_id VARCHAR(32) NOT NULL,
    webhook_url VARCHAR(500) NOT NULL,
    secret VARCHAR(64),
    events VARCHAR(200) DEFAULT 'success,failed',
    status INT DEFAULT 1,
    retry_count INT DEFAULT 3,
    timeout INT DEFAULT 5,
    description VARCHAR(200),
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_webhook_configs_app_id (app_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE callback_logs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    task_id VARCHAR(36),
    app_id VARCHAR(32) NOT NULL,
    provider_code VARCHAR(32) NOT NULL,
    provider_id VARCHAR(64),
    callback_status VARCHAR(20),
    error_code VARCHAR(32),
    error_message TEXT,
    raw_data JSON,
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_callback_logs_task_id (task_id),
    KEY idx_callback_logs_app_id (app_id),
    KEY idx_callback_logs_provider_code (provider_code),
    KEY idx_callback_logs_provider_id (provider_id),
    KEY idx_callback_logs_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE webhook_logs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    task_id VARCHAR(36),
    app_id VARCHAR(32) NOT NULL,
    webhook_config_id BIGINT UNSIGNED,
    webhook_url VARCHAR(500) NOT NULL,
    event VARCHAR(20) NOT NULL,
    request_data JSON,
    response_status INT,
    response_data TEXT,
    status VARCHAR(20) NOT NULL,
    error_message TEXT,
    retry_count INT DEFAULT 0,
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_webhook_logs_task_id (task_id),
    KEY idx_webhook_logs_app_id (app_id),
    KEY idx_webhook_logs_webhook_config (webhook_config_id),
    KEY idx_webhook_logs_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE failure_rules (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    scene VARCHAR(20) NOT NULL,
    provider_code VARCHAR(50),
    message_type VARCHAR(20),
    error_code VARCHAR(200),
    error_keyword VARCHAR(200),
    action VARCHAR(20) NOT NULL,
    action_config JSON,
    priority INT DEFAULT 0,
    status TINYINT DEFAULT 1,
    remark VARCHAR(500),
    created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    KEY idx_failure_rules_scene_priority (scene, priority),
    KEY idx_failure_rules_provider_code (provider_code),
    KEY idx_failure_rules_message_type (message_type),
    KEY idx_failure_rules_status (status),
    KEY idx_failure_rules_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
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
