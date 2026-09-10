package migration

import (
	"fmt"
	"strings"
	"testing"

	migrationFS "cnb.cool/mliev/push/message-push/migrations"
)

func TestProviderSignatureCodeExpansionMigrationAcrossDialects(t *testing.T) {
	const migrationName = "20260806000001_expand_provider_signature_code.sql"

	for _, dialect := range []string{"mysql", "pgsql", "sqlite"} {
		t.Run(dialect, func(t *testing.T) {
			path := fmt.Sprintf("%s/%s", dialect, migrationName)
			contents, err := migrationFS.FS().ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			normalizedSQL := strings.Join(strings.Fields(strings.ToLower(string(contents))), " ")
			for _, fragment := range []string{"-- +goose up", "-- +goose down"} {
				if !strings.Contains(normalizedSQL, fragment) {
					t.Errorf("%s missing migration contract fragment %q", path, fragment)
				}
			}
			if dialect != "sqlite" {
				for _, fragment := range []string{"alter table provider_signatures", "signature_code", "varchar(200)"} {
					if !strings.Contains(normalizedSQL, fragment) {
						t.Errorf("%s missing migration contract fragment %q", path, fragment)
					}
				}
			}
		})
	}
}
