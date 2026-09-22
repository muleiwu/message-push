package service

import (
	"os"
	"testing"
	"time"
)

// TestMain pins the process time zone to UTC, matching main.go's
// `time.Local = time.UTC`. Without it, timestamps written by these tests carry
// the host's local offset while the statistics queries compare against UTC
// instants, so the range filter finds nothing outside a UTC host.
func TestMain(m *testing.M) {
	time.Local = time.UTC
	os.Exit(m.Run())
}