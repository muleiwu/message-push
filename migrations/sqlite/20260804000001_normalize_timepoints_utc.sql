-- +goose Up
-- SQLite has no timezone-aware datetime type. Canonicalize all time-point
-- values to millisecond UTC text. Naive historical values are interpreted as
-- UTC; values with an explicit offset are converted to the same instant.
UPDATE applications SET
    created_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', created_at), '0'), '.') || '+00:00',
    updated_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', updated_at), '0'), '.') || '+00:00',
    deleted_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', deleted_at), '0'), '.') || '+00:00';
UPDATE provider_accounts SET
    created_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', created_at), '0'), '.') || '+00:00',
    updated_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', updated_at), '0'), '.') || '+00:00',
    deleted_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', deleted_at), '0'), '.') || '+00:00';
UPDATE provider_signatures SET
    created_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', created_at), '0'), '.') || '+00:00',
    updated_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', updated_at), '0'), '.') || '+00:00',
    deleted_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', deleted_at), '0'), '.') || '+00:00';
UPDATE channels SET
    created_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', created_at), '0'), '.') || '+00:00',
    updated_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', updated_at), '0'), '.') || '+00:00',
    deleted_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', deleted_at), '0'), '.') || '+00:00';
UPDATE push_tasks SET
    callback_time = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', callback_time), '0'), '.') || '+00:00',
    scheduled_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', scheduled_at), '0'), '.') || '+00:00',
    created_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', created_at), '0'), '.') || '+00:00',
    updated_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', updated_at), '0'), '.') || '+00:00';
UPDATE push_batch_tasks SET
    created_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', created_at), '0'), '.') || '+00:00',
    updated_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', updated_at), '0'), '.') || '+00:00';
UPDATE push_logs SET created_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', created_at), '0'), '.') || '+00:00';
UPDATE message_templates SET
    created_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', created_at), '0'), '.') || '+00:00',
    updated_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', updated_at), '0'), '.') || '+00:00',
    deleted_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', deleted_at), '0'), '.') || '+00:00';
UPDATE provider_templates SET
    created_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', created_at), '0'), '.') || '+00:00',
    updated_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', updated_at), '0'), '.') || '+00:00',
    deleted_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', deleted_at), '0'), '.') || '+00:00';
UPDATE channel_template_bindings SET
    created_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', created_at), '0'), '.') || '+00:00',
    updated_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', updated_at), '0'), '.') || '+00:00',
    deleted_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', deleted_at), '0'), '.') || '+00:00';
UPDATE channel_signature_mappings SET
    created_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', created_at), '0'), '.') || '+00:00',
    updated_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', updated_at), '0'), '.') || '+00:00',
    deleted_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', deleted_at), '0'), '.') || '+00:00';
UPDATE channel_health_history SET
    check_time = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', check_time), '0'), '.') || '+00:00',
    created_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', created_at), '0'), '.') || '+00:00';
UPDATE app_quota_stats SET
    created_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', created_at), '0'), '.') || '+00:00',
    updated_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', updated_at), '0'), '.') || '+00:00';
UPDATE provider_quota_stats SET
    created_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', created_at), '0'), '.') || '+00:00',
    updated_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', updated_at), '0'), '.') || '+00:00';
UPDATE admin_users SET
    created_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', created_at), '0'), '.') || '+00:00',
    updated_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', updated_at), '0'), '.') || '+00:00',
    deleted_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', deleted_at), '0'), '.') || '+00:00';
UPDATE webhook_configs SET
    created_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', created_at), '0'), '.') || '+00:00',
    updated_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', updated_at), '0'), '.') || '+00:00';
UPDATE callback_logs SET created_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', created_at), '0'), '.') || '+00:00';
UPDATE webhook_logs SET
    next_attempt_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', next_attempt_at), '0'), '.') || '+00:00',
    locked_until = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', locked_until), '0'), '.') || '+00:00',
    created_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', created_at), '0'), '.') || '+00:00',
    updated_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', updated_at), '0'), '.') || '+00:00';
UPDATE failure_rules SET
    created_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', created_at), '0'), '.') || '+00:00',
    updated_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', updated_at), '0'), '.') || '+00:00',
    deleted_at = rtrim(rtrim(strftime('%Y-%m-%d %H:%M:%f', deleted_at), '0'), '.') || '+00:00';

-- +goose Down
-- UTC normalization is intentionally irreversible; restore from the pre-upgrade
-- backup if the original SQLite text representation is required.
SELECT 1;
