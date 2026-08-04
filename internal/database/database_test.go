package database

import (
	"strings"
	"testing"
	"time"
)

func TestMySQLConnectionConfigPinsUTC(t *testing.T) {
	cfg, err := MySQLConfig(Config{
		Host: "127.0.0.1", Port: 3306, DBName: "push", Username: "user", Password: "pass",
	})
	if err != nil {
		t.Fatalf("build MySQL config: %v", err)
	}
	if cfg.Loc != time.UTC {
		t.Fatalf("MySQL location = %v, want UTC", cfg.Loc)
	}
	if got := cfg.Params["time_zone"]; got != "'+00:00'" {
		t.Fatalf("MySQL session time_zone = %q, want quoted +00:00", got)
	}
	if !cfg.MultiStatements {
		t.Fatal("MySQL connection must allow Goose multi-statement migrations")
	}
	formatted := cfg.FormatDSN()
	// UTC is the driver's default location, so FormatDSN intentionally omits
	// loc=UTC; the Config.Loc assertion above verifies the driver behavior.
	if !strings.Contains(formatted, "time_zone=%27%2B00%3A00%27") {
		t.Fatalf("MySQL DSN does not pin UTC: %s", formatted)
	}
}

func TestPostgresConnectionConfigPinsUTC(t *testing.T) {
	dsn, err := PostgreSQLDSN(Config{
		Host: "127.0.0.1", Port: 5432, DBName: "push", Username: "user", Password: "pass",
	})
	if err != nil {
		t.Fatalf("build PostgreSQL DSN: %v", err)
	}
	if !strings.Contains(dsn, "TimeZone=UTC") {
		t.Fatalf("PostgreSQL DSN does not pin UTC: %s", dsn)
	}
}

func TestGORMClockReturnsUTC(t *testing.T) {
	now := GORMConfig().NowFunc()
	if now.Location() != time.UTC {
		t.Fatalf("GORM NowFunc location = %v, want UTC", now.Location())
	}
}

func TestSQLiteConnectionConfigPinsUTC(t *testing.T) {
	dsn := sqliteUTCDataSource("/tmp/message-push.db")
	if !strings.Contains(dsn, "_time_format=sqlite") || !strings.Contains(dsn, "_timezone=UTC") {
		t.Fatalf("SQLite DSN does not pin UTC time formatting: %s", dsn)
	}
}
