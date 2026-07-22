package migration

import (
	"fmt"
	"strings"
	"testing"

	migrationFS "cnb.cool/mliev/push/message-push/migrations"
)

func TestAdminEmailMigrationContractAcrossDialects(t *testing.T) {
	const migrationName = "20260723000001_unique_admin_user_email.sql"
	requiredFragments := []string{
		"-- +goose up",
		"set email = lower(trim(email))",
		"set email = null",
		"create unique index",
		"on admin_users(email)",
		"update admin_users set status = 0 where status = 2",
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
