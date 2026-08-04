//go:build integration

package migration

import (
	"fmt"
	"os"
	"testing"
	"time"

	appDatabase "cnb.cool/mliev/push/message-push/internal/database"
	migrationFS "cnb.cool/mliev/push/message-push/migrations"
	"github.com/pressly/goose/v3"
)

func TestPostgreSQLTimepointsAreTimestamptz(t *testing.T) {
	requireTimezoneIntegration(t)
	port := integrationPort("POSTGRES_TEST_PORT", 55432)
	db, err := appDatabase.Open(appDatabase.Config{
		Driver: "postgres", Host: "127.0.0.1", Port: port, DBName: "message_push_test",
		Username: "message_push", Password: "message_push",
	})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("postgres sql DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	goose.SetBaseFS(migrationFS.FS())
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set postgres dialect: %v", err)
	}
	if err := goose.UpTo(sqlDB, migrationFS.DialectDir("postgres"), preUTCMigrationVersion); err != nil {
		t.Fatalf("migrate postgres to pre-UTC version: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO applications (app_id, app_secret, app_name, status, created_at, updated_at) VALUES ('utc-history', 'secret', 'UTC History', 1, TIMESTAMP '2026-08-04 10:00:00', TIMESTAMP '2026-08-04 10:00:00')`); err != nil {
		t.Fatalf("seed postgres history: %v", err)
	}
	if err := RunGooseMigrations(sqlDB, "postgres", nil); err != nil {
		t.Fatalf("run postgres UTC migration: %v", err)
	}
	var sessionTimezone string
	if err := sqlDB.QueryRow(`SHOW TIME ZONE`).Scan(&sessionTimezone); err != nil {
		t.Fatalf("query postgres session timezone: %v", err)
	}
	if sessionTimezone != "UTC" {
		t.Fatalf("postgres session timezone = %q", sessionTimezone)
	}

	for _, item := range utcInstantColumns {
		var dataType string
		if err := sqlDB.QueryRow(`SELECT data_type FROM information_schema.columns WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2`, item.table, item.column).Scan(&dataType); err != nil {
			t.Fatalf("query postgres type %s.%s: %v", item.table, item.column, err)
		}
		if dataType != "timestamp with time zone" {
			t.Errorf("postgres %s.%s type = %q", item.table, item.column, dataType)
		}
	}
	var historicalUTC string
	if err := sqlDB.QueryRow(`SELECT to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS') FROM applications WHERE app_id = 'utc-history'`).Scan(&historicalUTC); err != nil {
		t.Fatalf("read postgres historical instant: %v", err)
	}
	if historicalUTC != "2026-08-04 10:00:00" {
		t.Fatalf("postgres historical UTC = %q", historicalUTC)
	}

	if err := goose.DownTo(sqlDB, migrationFS.DialectDir("postgres"), preUTCMigrationVersion); err != nil {
		t.Fatalf("roll back postgres UTC migration: %v", err)
	}
	for _, item := range utcInstantColumns {
		var dataType string
		if err := sqlDB.QueryRow(`SELECT data_type FROM information_schema.columns WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2`, item.table, item.column).Scan(&dataType); err != nil {
			t.Fatalf("query rolled-back postgres type %s.%s: %v", item.table, item.column, err)
		}
		if dataType != "timestamp without time zone" {
			t.Errorf("rolled-back postgres %s.%s type = %q", item.table, item.column, dataType)
		}
	}
	if err := sqlDB.QueryRow(`SELECT to_char(created_at, 'YYYY-MM-DD HH24:MI:SS') FROM applications WHERE app_id = 'utc-history'`).Scan(&historicalUTC); err != nil {
		t.Fatalf("read rolled-back postgres historical value: %v", err)
	}
	if historicalUTC != "2026-08-04 10:00:00" {
		t.Fatalf("rolled-back postgres historical UTC = %q", historicalUTC)
	}
}

func TestMySQLSessionAndOffsetRoundTripUseUTC(t *testing.T) {
	requireTimezoneIntegration(t)
	port := integrationPort("MYSQL_TEST_PORT", 53306)
	db, err := appDatabase.Open(appDatabase.Config{
		Driver: "mysql", Host: "127.0.0.1", Port: port, DBName: "message_push_test",
		Username: "message_push", Password: "message_push",
	})
	if err != nil {
		t.Fatalf("open mysql: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("mysql sql DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := RunGooseMigrations(sqlDB, "mysql", nil); err != nil {
		t.Fatalf("run mysql migrations: %v", err)
	}

	var sessionTimezone string
	if err := sqlDB.QueryRow(`SELECT @@session.time_zone`).Scan(&sessionTimezone); err != nil {
		t.Fatalf("query mysql session timezone: %v", err)
	}
	if sessionTimezone != "+00:00" {
		t.Fatalf("mysql session time_zone = %q", sessionTimezone)
	}

	input := time.Date(2026, 8, 4, 18, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	if _, err := sqlDB.Exec(`INSERT INTO applications (app_id, app_secret, app_name, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, "offset-roundtrip", "secret", "Offset Roundtrip", 1, input, input); err != nil {
		t.Fatalf("insert mysql offset instant: %v", err)
	}
	var readBack time.Time
	if err := sqlDB.QueryRow(`SELECT created_at FROM applications WHERE app_id = ?`, "offset-roundtrip").Scan(&readBack); err != nil {
		t.Fatalf("read mysql offset instant: %v", err)
	}
	if !readBack.Equal(time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)) || readBack.Location() != time.UTC {
		t.Fatalf("mysql round trip = %s (%s)", readBack.Format(time.RFC3339), readBack.Location())
	}
}

func requireTimezoneIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("TIMEZONE_INTEGRATION") != "1" {
		t.Skip("set TIMEZONE_INTEGRATION=1 to run database integration tests")
	}
}

func integrationPort(name string, fallback int) int {
	var port int
	if _, err := fmt.Sscanf(os.Getenv(name), "%d", &port); err == nil && port > 0 {
		return port
	}
	return fallback
}
