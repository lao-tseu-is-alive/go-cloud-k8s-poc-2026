package module

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestPendingVersions(t *testing.T) {
	got := pendingVersions([]string{"0001", "0002", "0003"}, []string{"0001", "0003", "9999"})
	if !slices.Equal(got, []string{"0002"}) {
		t.Fatalf("pending = %v, want [0002]", got)
	}
	if got := pendingVersions([]string{"0001"}, []string{"0001"}); len(got) != 0 {
		t.Fatalf("an up-to-date schema has nothing pending: %v", got)
	}
}

func TestEmbeddedVersionsAreOrderedAndUnique(t *testing.T) {
	versions, err := embeddedVersions()
	if err != nil || len(versions) == 0 {
		t.Fatalf("embedded versions: %v (%v)", versions, err)
	}
	if !slices.IsSorted(versions) || len(slices.Compact(slices.Clone(versions))) != len(versions) {
		t.Fatalf("versions must be sorted and unique: %v", versions)
	}
}

func TestValidRoleName(t *testing.T) {
	for _, ok := range []string{"goeland_app", "_r", "app2"} {
		if !ValidRoleName(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "App", "2app", "app-role", `app"; DROP`, strings.Repeat("a", 64)} {
		if ValidRoleName(bad) {
			t.Errorf("%q should be refused", bad)
		}
	}
}

// deleteStatement finds the table of a DELETE in Go source.
var deleteStatement = regexp.MustCompile(`(?i)\bDELETE\s+FROM\s+([a-z_][a-z0-9_]*)`)

// TestRuntimeDeletesAreGranted scans the server's Go sources: every table the
// application deletes from must be in RuntimeDeletableTables, or the runtime
// role (GLD-046) would be refused the statement in production. The legacy
// import runs as the schema owner and is not scanned.
func TestRuntimeDeletesAreGranted(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	found := map[string]string{}
	for _, dir := range []string{"pkg", filepath.Join("cmd", "goeland-server")} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && (d.Name() == "node_modules" || d.Name() == "legacyimport" || d.Name() == "integration") {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, m := range deleteStatement.FindAllStringSubmatch(string(src), -1) {
				found[strings.ToLower(m[1])] = path
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(found) == 0 {
		t.Fatal("no DELETE found: the scan is broken")
	}
	for table, path := range found {
		if !slices.Contains(RuntimeDeletableTables, table) {
			t.Errorf("%s deletes from %s, which RuntimeDeletableTables does not grant", path, table)
		}
	}
}
