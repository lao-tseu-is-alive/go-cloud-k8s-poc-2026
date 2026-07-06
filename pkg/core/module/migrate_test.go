package module

import (
	"io/fs"
	"strings"
	"testing"
)

// TestEmbeddedMigrationsParse ensures every embedded migration exposes a valid
// "-- migrate:up" section that the parser can turn into executable statements.
func TestEmbeddedMigrationsParse(t *testing.T) {
	paths, err := fs.Glob(Migrations, "db/migrations/*.sql")
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no embedded migrations found")
	}
	for _, path := range paths {
		content, err := Migrations.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		statements, err := ParseDBMateUp(string(content))
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if len(statements) == 0 {
			t.Fatalf("%s produced no statements", path)
		}
		for _, s := range statements {
			if strings.TrimSpace(s) == "" {
				t.Fatalf("%s produced a blank statement", path)
			}
		}
	}
}

func TestParseDBMateUpKeepsFunctionBlockTogether(t *testing.T) {
	content := `-- migrate:up
CREATE TABLE t (id int);
-- migrate:statementbegin
CREATE FUNCTION f() RETURNS trigger AS $$
BEGIN
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- migrate:statementend
CREATE TRIGGER trg BEFORE UPDATE ON t FOR EACH ROW EXECUTE FUNCTION f();
-- migrate:down
DROP TABLE t;`

	statements, err := ParseDBMateUp(content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(statements) != 3 {
		t.Fatalf("expected 3 statements (table, function block, trigger), got %d: %#v", len(statements), statements)
	}
	if !strings.Contains(statements[1], "CREATE FUNCTION") || !strings.Contains(statements[1], "LANGUAGE plpgsql") {
		t.Fatalf("function block was split: %q", statements[1])
	}
	// The down section must be excluded from the up statements.
	for _, s := range statements {
		if strings.Contains(s, "DROP TABLE") {
			t.Fatalf("down statement leaked into up: %q", s)
		}
	}
}

func TestMigrationVersion(t *testing.T) {
	cases := map[string]string{
		"db/migrations/0001_subject_core.sql":        "0001",
		"db/migrations/0004_seed_reference_data.sql": "0004",
		"0002_relationships.sql":                     "0002",
	}
	for path, want := range cases {
		if got := migrationVersion(path); got != want {
			t.Fatalf("migrationVersion(%q) = %q, want %q", path, got, want)
		}
	}
}
