package coretest

import (
	"os"
	"strings"
	"testing"
)

const (
	// DatabaseURLEnv names the DSN of the disposable PostGIS database that
	// enables the database integration and end-to-end tests.
	DatabaseURLEnv = "GOELAND_TEST_DATABASE_URL"
	// RequireDatabaseEnv, set to "true" (CI does), turns a missing DSN into a
	// failure instead of a skip, so those tests can never pass by not running.
	RequireDatabaseEnv = "GOELAND_REQUIRE_DB_TESTS"
)

// TestDatabaseURL returns the test database DSN. Without one the test is
// skipped, or fails when RequireDatabaseEnv is "true". what names the tests
// in the message ("DB integration tests").
func TestDatabaseURL(t *testing.T, what string) string {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(DatabaseURLEnv))
	if dsn != "" {
		return dsn
	}
	if os.Getenv(RequireDatabaseEnv) == "true" {
		t.Fatalf("%s=true but %s is unset: %s cannot run", RequireDatabaseEnv, DatabaseURLEnv, what)
	}
	t.Skipf("set %s to run the %s (needs PostGIS, pgcrypto, pg_trgm, unaccent)", DatabaseURLEnv, what)
	return ""
}
