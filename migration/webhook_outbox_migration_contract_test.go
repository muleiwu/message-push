package migration

import (
	"fmt"
	"strings"
	"testing"

	migrationFS "cnb.cool/mliev/push/message-push/migrations"
)

func TestWebhookOutboxMigrationContractAcrossDialects(t *testing.T) {
	const migrationName = "20260730000001_webhook_outbox.sql"
	requiredFragments := []string{
		"-- +goose up",
		"alter table webhook_logs",
		"dedup_key",
		"signing_secret",
		"max_retries",
		"timeout_seconds",
		"next_attempt_at",
		"locked_until",
		"lease_token",
		"updated_at",
		"create unique index uk_webhook_logs_dedup_key",
		"create index idx_webhook_logs_dispatch",
		"-- +goose down",
	}

	for _, dialect := range []string{"mysql", "pgsql", "sqlite"} {
		t.Run(dialect, func(t *testing.T) {
			path := fmt.Sprintf("%s/%s", dialect, migrationName)
			contents, err := migrationFS.FS().ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			normalizedSQL := strings.Join(strings.Fields(strings.ToLower(string(contents))), " ")
			for _, fragment := range requiredFragments {
				if !strings.Contains(normalizedSQL, fragment) {
					t.Errorf("%s missing migration contract fragment %q", path, fragment)
				}
			}
		})
	}
}
