package migration

import (
	"fmt"
	"strings"
	"testing"

	migrationFS "cnb.cool/mliev/push/message-push/migrations"
)

func TestEmailAttachmentMigrationContractAcrossDialects(t *testing.T) {
	const migrationName = "20260827000001_add_email_attachments.sql"
	requiredFragments := []string{
		"-- +goose up",
		"create table email_attachments",
		"attachment_group_id",
		"content",
		"purged_at",
		"alter table push_tasks",
		"idx_push_tasks_attachment_group",
		"-- +goose down",
		"drop table email_attachments",
	}
	for _, dialect := range []string{"mysql", "pgsql", "sqlite"} {
		t.Run(dialect, func(t *testing.T) {
			path := fmt.Sprintf("%s/%s", dialect, migrationName)
			contents, err := migrationFS.FS().ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			normalized := strings.Join(strings.Fields(strings.ToLower(string(contents))), " ")
			for _, fragment := range requiredFragments {
				if !strings.Contains(normalized, fragment) {
					t.Errorf("%s missing %q", path, fragment)
				}
			}
		})
	}
}
