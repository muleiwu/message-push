package migration

import (
	"fmt"
	"strings"
	"testing"

	migrationFS "cnb.cool/mliev/push/message-push/migrations"
)

func TestLastProviderMigrationContractAcrossDialects(t *testing.T) {
	const migrationName = "20260730000002_add_last_provider_to_push_tasks.sql"
	requiredFragments := []string{
		"-- +goose up",
		"alter table push_tasks",
		"provider_account_id",
		"update push_tasks",
		"order by push_logs.id desc",
		"create index idx_push_tasks_provider_account",
		"-- +goose down",
		"drop column provider_account_id",
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
