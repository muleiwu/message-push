package migration

import (
	"fmt"
	"strings"
	"testing"

	migrationFS "cnb.cool/mliev/push/message-push/migrations"
)

type schemaColumn struct {
	table  string
	column string
}

const preUTCMigrationVersion = int64(20260730000002)

var utcInstantColumns = []schemaColumn{
	{"applications", "created_at"}, {"applications", "updated_at"}, {"applications", "deleted_at"},
	{"provider_accounts", "created_at"}, {"provider_accounts", "updated_at"}, {"provider_accounts", "deleted_at"},
	{"provider_signatures", "created_at"}, {"provider_signatures", "updated_at"}, {"provider_signatures", "deleted_at"},
	{"channels", "created_at"}, {"channels", "updated_at"}, {"channels", "deleted_at"},
	{"push_tasks", "callback_time"}, {"push_tasks", "scheduled_at"}, {"push_tasks", "created_at"}, {"push_tasks", "updated_at"},
	{"push_batch_tasks", "created_at"}, {"push_batch_tasks", "updated_at"},
	{"push_logs", "created_at"},
	{"message_templates", "created_at"}, {"message_templates", "updated_at"}, {"message_templates", "deleted_at"},
	{"provider_templates", "created_at"}, {"provider_templates", "updated_at"}, {"provider_templates", "deleted_at"},
	{"channel_template_bindings", "created_at"}, {"channel_template_bindings", "updated_at"}, {"channel_template_bindings", "deleted_at"},
	{"channel_signature_mappings", "created_at"}, {"channel_signature_mappings", "updated_at"}, {"channel_signature_mappings", "deleted_at"},
	{"channel_health_history", "check_time"}, {"channel_health_history", "created_at"},
	{"app_quota_stats", "created_at"}, {"app_quota_stats", "updated_at"},
	{"provider_quota_stats", "created_at"}, {"provider_quota_stats", "updated_at"},
	{"admin_users", "created_at"}, {"admin_users", "updated_at"}, {"admin_users", "deleted_at"},
	{"webhook_configs", "created_at"}, {"webhook_configs", "updated_at"},
	{"callback_logs", "created_at"},
	{"webhook_logs", "next_attempt_at"}, {"webhook_logs", "locked_until"}, {"webhook_logs", "created_at"}, {"webhook_logs", "updated_at"},
	{"failure_rules", "created_at"}, {"failure_rules", "updated_at"}, {"failure_rules", "deleted_at"},
}

var businessDateColumns = []schemaColumn{
	{"app_quota_stats", "stat_date"},
	{"provider_quota_stats", "stat_date"},
}

func TestUTCTimepointMigrationCovers52ColumnInventory(t *testing.T) {
	if got := len(utcInstantColumns); got != 50 {
		t.Fatalf("UTC instant column count = %d, want 50", got)
	}
	if got := len(utcInstantColumns) + len(businessDateColumns); got != 52 {
		t.Fatalf("time-related column inventory = %d, want 52", got)
	}

	for _, dialect := range []string{"pgsql", "sqlite"} {
		path := fmt.Sprintf("%s/20260804000001_normalize_timepoints_utc.sql", dialect)
		body, err := migrationFS.FS().ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		sqlText := string(body)
		for _, item := range utcInstantColumns {
			if !strings.Contains(sqlText, "ALTER TABLE "+item.table) && !strings.Contains(sqlText, "UPDATE "+item.table) {
				t.Errorf("%s does not include table %s", dialect, item.table)
			}
			if !strings.Contains(sqlText, item.column) {
				t.Errorf("%s does not include column %s.%s", dialect, item.table, item.column)
			}
		}
		for _, item := range businessDateColumns {
			if strings.Contains(sqlText, item.column+" =") || strings.Contains(sqlText, "COLUMN "+item.column+" TYPE") {
				t.Errorf("%s must not transform business date %s.%s", dialect, item.table, item.column)
			}
		}
	}
}
