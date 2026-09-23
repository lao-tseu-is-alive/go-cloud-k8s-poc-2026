package main

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestCheckAtlasAcceptsExactInventory(t *testing.T) {
	t.Parallel()

	files := []string{"README.md", "docs/atlas.md", "pkg/version/version.go"}
	source := fstest.MapFS{
		"README.md":              {Data: []byte("# Example\n")},
		"docs/atlas.md":          {Data: []byte(validAtlas())},
		"pkg/version/version.go": {Data: []byte("package version\nconst Version = \"1.2.3\"\n")},
	}
	findings, err := checkAtlas(source, files, defaultAtlasOptions())
	if err != nil {
		t.Fatalf("checkAtlas() error = %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("checkAtlas() findings = %v", findings)
	}
}

func TestCheckAtlasRejectsStructuralDrift(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		atlas    string
		files    []string
		wantText string
	}{
		{
			name:     "missing entry",
			atlas:    strings.Replace(validAtlas(), "- `README.md` — Project overview.\n", "", 1),
			files:    []string{"README.md", "docs/atlas.md", "pkg/version/version.go"},
			wantText: "missing entry for README.md",
		},
		{
			name:     "stale entry",
			atlas:    validAtlas() + "- `removed.go` — Removed file.\n",
			files:    []string{"README.md", "docs/atlas.md", "pkg/version/version.go"},
			wantText: "stale entry for removed.go",
		},
		{
			name:     "duplicate entry",
			atlas:    validAtlas() + "- `README.md` — Duplicate.\n",
			files:    []string{"README.md", "docs/atlas.md", "pkg/version/version.go"},
			wantText: "duplicate entry for README.md",
		},
		{
			name:     "wrong version",
			atlas:    strings.Replace(validAtlas(), "v1.2.3", "v1.2.2", 1),
			files:    []string{"README.md", "docs/atlas.md", "pkg/version/version.go"},
			wantText: "tracks v1.2.2, want v1.2.3",
		},
		{
			name:     "non canonical path",
			atlas:    strings.Replace(validAtlas(), "`README.md`", "`docs/../README.md`", 1),
			files:    []string{"README.md", "docs/atlas.md", "pkg/version/version.go"},
			wantText: "canonical repository-relative path",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			source := fstest.MapFS{
				"README.md":              {Data: []byte("# Example\n")},
				"docs/atlas.md":          {Data: []byte(test.atlas)},
				"pkg/version/version.go": {Data: []byte("package version\nconst Version = \"1.2.3\"\n")},
			}
			findings, err := checkAtlas(source, test.files, defaultAtlasOptions())
			if err != nil {
				t.Fatalf("checkAtlas() error = %v", err)
			}
			if !findFinding(findings, test.wantText) {
				t.Fatalf("checkAtlas() findings = %v, want text %q", findings, test.wantText)
			}
		})
	}
}

func TestCheckAtlasHonoursRepositoryConventions(t *testing.T) {
	t.Parallel()

	atlas := strings.Replace(validAtlas(), "Tracked version: **v1.2.3**.", "Version suivie : **v1.2.3**.", 1)
	atlas = strings.Replace(atlas, "`pkg/version/version.go`", "`internal/version/version.go`", 1)
	source := fstest.MapFS{
		"README.md":                   {Data: []byte("# Example\n")},
		"docs/atlas.md":               {Data: []byte(atlas)},
		"internal/version/version.go": {Data: []byte("package version\nconst Release = \"1.2.3\"\n")},
	}
	options := atlasOptions{
		path:         defaultAtlasPath,
		versionFile:  "internal/version/version.go",
		versionName:  "Release",
		bannerPrefix: "Version suivie : ",
	}
	files := []string{"README.md", "docs/atlas.md", "internal/version/version.go"}
	findings, err := checkAtlas(source, files, options)
	if err != nil {
		t.Fatalf("checkAtlas() error = %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("checkAtlas() findings = %v", findings)
	}

	// The default English banner must not match a French atlas.
	findings, err = checkAtlas(source, files, atlasOptions{
		path:         defaultAtlasPath,
		versionFile:  "internal/version/version.go",
		versionName:  "Release",
		bannerPrefix: defaultBannerPrefix,
	})
	if err != nil {
		t.Fatalf("checkAtlas() error = %v", err)
	}
	if !findFinding(findings, "missing tracked version banner") {
		t.Fatalf("checkAtlas() findings = %v, want missing banner", findings)
	}
}

func TestSourceVersionRejectsUnsafeDeclarations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		source   string
		wantText string
	}{
		{name: "variable", source: "package version\nvar Version = \"1.2.3\"\n", wantText: "must be a constant"},
		{name: "missing", source: "package version\nconst Other = \"1.2.3\"\n", wantText: "Version constant not found"},
		{name: "not a literal", source: "package version\nconst base = \"1\"\nconst Version = base\n", wantText: "not a string literal"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			source := fstest.MapFS{"pkg/version/version.go": {Data: []byte(test.source)}}
			_, err := sourceVersion(source, defaultVersionFile, defaultVersionName)
			if err == nil || !strings.Contains(err.Error(), test.wantText) {
				t.Fatalf("sourceVersion() error = %v, want text %q", err, test.wantText)
			}
		})
	}
}

func TestSourceVersionReadsConstantBlock(t *testing.T) {
	t.Parallel()

	source := fstest.MapFS{"pkg/version/version.go": {Data: []byte(`package version
const (
	AppName = "demo"
	Version = "0.4.0"
)
var Revision = "unknown"
`)}}
	version, err := sourceVersion(source, defaultVersionFile, defaultVersionName)
	if err != nil || version != "0.4.0" {
		t.Fatalf("sourceVersion() = %q, %v; want 0.4.0", version, err)
	}
}

func TestCheckGoSkipsMethodsOnPrivateTypes(t *testing.T) {
	t.Parallel()

	source := fstest.MapFS{
		"internal/example/example.go": {Data: []byte(`// Package example demonstrates the policy.
package example
type privateType struct{}
func (privateType) ExportedMethod() {}
`)},
	}
	findings, err := checkGo(source, []string{"internal/example/example.go"})
	if err != nil {
		t.Fatalf("checkGo() error = %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("checkGo() findings = %v", findings)
	}
}

func TestCheckGoAcceptsDocumentedAPIAndGeneratedFiles(t *testing.T) {
	t.Parallel()

	source := fstest.MapFS{
		"internal/example/example.go": {Data: []byte(`// Package example demonstrates the policy.
package example

// Config controls the example.
type Config struct {
	// Timeout bounds an operation.
	Timeout int
}

// Ready is the ready state.
const Ready = "ready"

// Open creates an example.
func Open() {}
`)},
		"internal/example/generated.go": {Data: []byte(`// Code generated by a test. DO NOT EDIT.
package example
type Undocumented struct{}
`)},
		"cmd/example/main.go": {Data: []byte(`// Command example demonstrates command documentation.
package main
func main() {}
`)},
	}
	files := []string{"cmd/example/main.go", "internal/example/example.go", "internal/example/generated.go"}
	findings, err := checkGo(source, files)
	if err != nil {
		t.Fatalf("checkGo() error = %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("checkGo() findings = %v", findings)
	}
}

func TestCheckGoReportsMissingContracts(t *testing.T) {
	t.Parallel()

	source := fstest.MapFS{
		"internal/example/example.go": {Data: []byte(`package example
type Config struct {
	Timeout int
}
const Ready = "ready"
func Open() {}
`)},
	}
	findings, err := checkGo(source, []string{"internal/example/example.go"})
	if err != nil {
		t.Fatalf("checkGo() error = %v", err)
	}
	for _, expected := range []string{
		"package example needs a package or command comment",
		"exported type Config",
		"exported field Timeout",
		"exported value Ready",
		"exported function or method Open",
	} {
		if !findFinding(findings, expected) {
			t.Errorf("checkGo() findings = %v, want text %q", findings, expected)
		}
	}
}

func validAtlas() string {
	return `# Atlas

Tracked version: **v1.2.3**.

- ` + "`README.md`" + ` — Project overview.
- ` + "`docs/atlas.md`" + ` — Repository atlas.
- ` + "`pkg/version/version.go`" + ` — Version source.
`
}

func findFinding(findings []finding, text string) bool {
	for _, item := range findings {
		if strings.Contains(item.String(), text) {
			return true
		}
	}
	return false
}
