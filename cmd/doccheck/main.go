// Command doccheck verifies the repository documentation contract described in
// docs/DOCUMENTATION.md without modifying the working tree.
//
// It checks two scopes over the Git inventory of tracked and untracked,
// non-ignored files:
//
//   - go: every non-generated package has a package or command comment, and
//     every exported identifier, method on an exported type and exported struct
//     field has a comment starting with its name;
//   - atlas: docs/atlas.md lists every inventory file exactly once, lists no
//     stale path, and its version banner matches the Version constant.
//
// The flags make the checker portable across repositories: the version source,
// the constant name and the banner wording are local conventions, not rules.
package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	defaultAtlasPath    = "docs/atlas.md"
	defaultVersionFile  = "pkg/version/version.go"
	defaultVersionName  = "Version"
	defaultBannerPrefix = "Tracked version: "
)

var atlasEntryPattern = regexp.MustCompile("^- `([^`]+)` — (.+)$")

// atlasOptions carries the repository-specific conventions of the atlas check.
type atlasOptions struct {
	path         string
	versionFile  string
	versionName  string
	bannerPrefix string
}

// bannerPattern matches "<prefix>**vX.Y.Z**." and captures the version.
func (options atlasOptions) bannerPattern() *regexp.Regexp {
	return regexp.MustCompile(`^` + regexp.QuoteMeta(options.bannerPrefix) + `\*\*v([0-9]+\.[0-9]+\.[0-9]+)\*\*\.$`)
}

func defaultAtlasOptions() atlasOptions {
	return atlasOptions{
		path:         defaultAtlasPath,
		versionFile:  defaultVersionFile,
		versionName:  defaultVersionName,
		bannerPrefix: defaultBannerPrefix,
	}
}

type finding struct {
	path    string
	line    int
	message string
}

func (item finding) String() string {
	if item.line > 0 {
		return fmt.Sprintf("%s:%d: %s", item.path, item.line, item.message)
	}
	return fmt.Sprintf("%s: %s", item.path, item.message)
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "doccheck: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	cfg, err := parseFlags(args)
	if err != nil {
		return err
	}
	files, err := repositoryFiles(ctx, cfg.root)
	if err != nil {
		return err
	}
	findings, err := collectFindings(os.DirFS(cfg.root), files, cfg)
	if err != nil {
		return err
	}
	sortFindings(findings)
	if len(findings) != 0 {
		for _, item := range findings {
			fmt.Fprintln(output, item)
		}
		return fmt.Errorf("documentation checks failed with %d finding(s)", len(findings))
	}
	fmt.Fprintf(output, "doccheck: %s OK\n", cfg.scope)
	return nil
}

// config is the parsed command line.
type config struct {
	root  string
	scope string
	atlas atlasOptions
}

// parseFlags parses and validates the command line.
func parseFlags(args []string) (config, error) {
	flags := flag.NewFlagSet("doccheck", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := flags.String("root", ".", "repository root")
	scope := flags.String("scope", "all", "checks to run: all, atlas, or go")
	atlasPath := flags.String("atlas", defaultAtlasPath, "atlas path relative to the repository root")
	versionFile := flags.String("version-file", defaultVersionFile, "Go file declaring the release version constant")
	versionName := flags.String("version-name", defaultVersionName, "name of the release version constant")
	bannerPrefix := flags.String("banner-prefix", defaultBannerPrefix, "atlas banner text preceding **vX.Y.Z**.")
	if err := flags.Parse(args); err != nil {
		return config{}, err
	}
	if flags.NArg() != 0 {
		return config{}, fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}
	if *scope != "all" && *scope != "atlas" && *scope != "go" {
		return config{}, fmt.Errorf("invalid scope %q (want all, atlas, or go)", *scope)
	}
	return config{
		root:  *root,
		scope: *scope,
		atlas: atlasOptions{path: *atlasPath, versionFile: *versionFile, versionName: *versionName, bannerPrefix: *bannerPrefix},
	}, nil
}

// collectFindings runs the checks selected by cfg.scope.
func collectFindings(source fs.FS, files []string, cfg config) ([]finding, error) {
	var findings []finding
	if cfg.scope == "all" || cfg.scope == "atlas" {
		atlasFindings, err := checkAtlas(source, files, cfg.atlas)
		if err != nil {
			return nil, err
		}
		findings = append(findings, atlasFindings...)
	}
	if cfg.scope == "all" || cfg.scope == "go" {
		goFindings, err := checkGo(source, files)
		if err != nil {
			return nil, err
		}
		findings = append(findings, goFindings...)
	}
	return findings, nil
}

func repositoryFiles(ctx context.Context, root string) ([]string, error) {
	// git is resolved from PATH on purpose: doccheck is a developer and CI tool
	// run inside the repository's own toolchain, not a privileged service.
	command := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard") // NOSONAR go:S4036
	data, err := command.Output()
	if err != nil {
		return nil, errors.New("list repository files with git")
	}
	parts := bytes.Split(data, []byte{0})
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		files = append(files, filepath.ToSlash(string(part)))
	}
	sort.Strings(files)
	return files, nil
}

func checkAtlas(source fs.FS, files []string, options atlasOptions) ([]finding, error) {
	content, err := fs.ReadFile(source, options.path)
	if err != nil {
		return nil, fmt.Errorf("read atlas: %w", err)
	}
	version, err := sourceVersion(source, options.versionFile, options.versionName)
	if err != nil {
		return nil, err
	}
	parsed, err := parseAtlas(content, options)
	if err != nil {
		return nil, err
	}
	findings := parsed.findings
	if banner := checkBanner(options.path, parsed.version, version); banner != nil {
		findings = append(findings, *banner)
	}
	return append(findings, compareInventory(options.path, parsed.entries, files)...), nil
}

// parsedAtlas is the result of reading atlas.md line by line.
type parsedAtlas struct {
	entries  map[string]int // path -> declaring line
	version  string         // version of the banner, empty when missing
	findings []finding      // malformed or duplicate entries
}

// parseAtlas reads the banner and every "- `path` — description" entry.
func parseAtlas(content []byte, options atlasOptions) (parsedAtlas, error) {
	result := parsedAtlas{entries: make(map[string]int)}
	bannerPattern := options.bannerPattern()
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := scanner.Text()
		if matches := bannerPattern.FindStringSubmatch(line); matches != nil {
			result.version = matches[1]
		}
		if !strings.HasPrefix(line, "- `") {
			continue
		}
		path, problem := parseAtlasEntry(line)
		if problem == "" {
			if previous, exists := result.entries[path]; exists {
				problem = fmt.Sprintf("duplicate entry for %s (first declared at line %d)", path, previous)
			}
		}
		if problem != "" {
			result.findings = append(result.findings, finding{path: options.path, line: lineNumber, message: problem})
			continue
		}
		result.entries[path] = lineNumber
	}
	if err := scanner.Err(); err != nil {
		return parsedAtlas{}, fmt.Errorf("scan atlas: %w", err)
	}
	return result, nil
}

// parseAtlasEntry returns the canonical path of an entry line, or a problem
// description when the line is malformed.
func parseAtlasEntry(line string) (string, string) {
	matches := atlasEntryPattern.FindStringSubmatch(line)
	if matches == nil {
		return "", "invalid atlas entry; want - `path` — description"
	}
	path := filepath.ToSlash(matches[1])
	if strings.TrimSpace(matches[2]) == "" {
		return "", "atlas entry needs a non-empty description"
	}
	if path != matches[1] || path == "." || strings.HasPrefix(path, "../") || filepath.Clean(path) != path {
		return "", "atlas entry path must be a canonical repository-relative path"
	}
	return path, ""
}

// checkBanner compares the tracked version banner with the source version.
func checkBanner(atlasPath, atlasVersion, version string) *finding {
	switch {
	case atlasVersion == "":
		return &finding{path: atlasPath, message: "missing tracked version banner"}
	case atlasVersion != version:
		return &finding{path: atlasPath, message: fmt.Sprintf("tracks v%s, want v%s", atlasVersion, version)}
	}
	return nil
}

// compareInventory reports repository files missing from the atlas and atlas
// entries that no longer exist.
func compareInventory(atlasPath string, entries map[string]int, files []string) []finding {
	var findings []finding
	repositorySet := make(map[string]struct{}, len(files))
	for _, path := range files {
		repositorySet[path] = struct{}{}
		if _, exists := entries[path]; !exists {
			findings = append(findings, finding{path: atlasPath, message: "missing entry for " + path})
		}
	}
	for path, line := range entries {
		if _, exists := repositorySet[path]; !exists {
			findings = append(findings, finding{path: atlasPath, line: line, message: "stale entry for " + path})
		}
	}
	return findings
}

func checkGo(source fs.FS, files []string) ([]finding, error) {
	packages := make(map[string]*packageState)
	var findings []finding
	for _, path := range files {
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			continue
		}
		fileFindings, err := checkGoFile(source, path, packages)
		if err != nil {
			return nil, err
		}
		findings = append(findings, fileFindings...)
	}
	for _, state := range packages {
		if !state.documented {
			findings = append(findings, finding{path: state.firstPath, message: fmt.Sprintf("package %s needs a package or command comment", state.name)})
		}
	}
	return findings, nil
}

// packageState tracks whether a package has a package comment in any file.
type packageState struct {
	name       string
	documented bool
	firstPath  string
}

// checkGoFile checks one non-generated Go file and records its package state.
func checkGoFile(source fs.FS, path string, packages map[string]*packageState) ([]finding, error) {
	content, err := fs.ReadFile(source, path)
	if err != nil {
		return nil, fmt.Errorf("read Go source %s: %w", path, err)
	}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse Go source %s: %w", path, err)
	}
	if ast.IsGenerated(file) {
		return nil, nil
	}
	directory := filepath.ToSlash(filepath.Dir(path))
	state, exists := packages[directory]
	if !exists {
		state = &packageState{name: file.Name.Name, firstPath: path}
		packages[directory] = state
	}
	if validPackageComment(file.Name.Name, commentText(file.Doc)) {
		state.documented = true
	}
	var findings []finding
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			if declaration.Name.IsExported() && receiverIsDocumentedAPI(declaration.Recv) && !commentStartsWith(declaration.Doc, declaration.Name.Name) {
				findings = append(findings, missingDoc(fileSet, path, declaration.Pos(), "exported function or method", declaration.Name.Name))
			}
		case *ast.GenDecl:
			findings = append(findings, checkGeneralDeclaration(fileSet, path, declaration)...)
		}
	}
	return findings, nil
}

func receiverIsDocumentedAPI(receiver *ast.FieldList) bool {
	if receiver == nil || len(receiver.List) == 0 {
		return true
	}
	typeExpression := receiver.List[0].Type
	if pointer, ok := typeExpression.(*ast.StarExpr); ok {
		typeExpression = pointer.X
	}
	identifier, ok := typeExpression.(*ast.Ident)
	return ok && identifier.IsExported()
}

func checkGeneralDeclaration(fileSet *token.FileSet, path string, declaration *ast.GenDecl) []finding {
	if declaration.Tok != token.TYPE && declaration.Tok != token.CONST && declaration.Tok != token.VAR {
		return nil
	}
	var findings []finding
	for _, rawSpec := range declaration.Specs {
		switch spec := rawSpec.(type) {
		case *ast.TypeSpec:
			findings = append(findings, checkTypeSpec(fileSet, path, declaration, spec)...)
		case *ast.ValueSpec:
			findings = append(findings, checkValueSpec(fileSet, path, declaration, spec)...)
		}
	}
	return findings
}

// checkTypeSpec requires a comment on an exported type and on every exported
// field of a struct type.
func checkTypeSpec(fileSet *token.FileSet, path string, declaration *ast.GenDecl, spec *ast.TypeSpec) []finding {
	var findings []finding
	if spec.Name.IsExported() && !commentStartsWith(specDoc(spec.Doc, declaration), spec.Name.Name) {
		findings = append(findings, missingDoc(fileSet, path, spec.Pos(), "exported type", spec.Name.Name))
	}
	structure, ok := spec.Type.(*ast.StructType)
	if !ok {
		return findings
	}
	for _, field := range structure.Fields.List {
		for _, name := range field.Names {
			if name.IsExported() && !commentStartsWithEither(field.Doc, field.Comment, name.Name) {
				findings = append(findings, positionFinding(fileSet, path, name.Pos(), "exported field "+name.Name+" needs a comment starting with its name"))
			}
		}
	}
	return findings
}

// checkValueSpec requires a comment on every exported constant or variable.
func checkValueSpec(fileSet *token.FileSet, path string, declaration *ast.GenDecl, spec *ast.ValueSpec) []finding {
	var findings []finding
	doc := specDoc(spec.Doc, declaration)
	for _, name := range spec.Names {
		if name.IsExported() && !commentStartsWithEither(doc, spec.Comment, name.Name) {
			findings = append(findings, missingDoc(fileSet, path, name.Pos(), "exported value", name.Name))
		}
	}
	return findings
}

// specDoc returns the spec's own doc comment, or the enclosing declaration's
// for a single ungrouped declaration.
func specDoc(doc *ast.CommentGroup, declaration *ast.GenDecl) *ast.CommentGroup {
	if doc == nil {
		return declaration.Doc
	}
	return doc
}

// missingGoDocSuffix ends every "needs a GoDoc comment" finding message.
const missingGoDocSuffix = " needs a GoDoc comment starting with its name"

// missingDoc reports an exported identifier without a comment starting with its name.
func missingDoc(fileSet *token.FileSet, path string, position token.Pos, what, name string) finding {
	return positionFinding(fileSet, path, position, what+" "+name+missingGoDocSuffix)
}

func validPackageComment(packageName, text string) bool {
	if packageName == "main" {
		return strings.HasPrefix(text, "Command ") || strings.HasPrefix(text, "Package main ")
	}
	return strings.HasPrefix(text, "Package "+packageName+" ")
}

func commentStartsWith(group *ast.CommentGroup, name string) bool {
	return group != nil && startsWithName(commentText(group), name)
}

func commentStartsWithEither(first, second *ast.CommentGroup, name string) bool {
	return commentStartsWith(first, name) || commentStartsWith(second, name)
}

func commentText(group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}
	return strings.TrimSpace(group.Text())
}

func startsWithName(text, name string) bool {
	return text == name || strings.HasPrefix(text, name+" ") || strings.HasPrefix(text, name+".") || strings.HasPrefix(text, name+",")
}

func positionFinding(fileSet *token.FileSet, path string, position token.Pos, message string) finding {
	return finding{path: path, line: fileSet.Position(position).Line, message: message}
}

// sourceVersion returns the string literal assigned to the constant name in
// path. A variable is rejected because -ldflags -X could silently override it,
// letting a binary report a version different from the audited source.
func sourceVersion(source fs.FS, path, name string) (string, error) {
	content, err := fs.ReadFile(source, path)
	if err != nil {
		return "", fmt.Errorf("read version source: %w", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, content, 0)
	if err != nil {
		return "", fmt.Errorf("parse version source: %w", err)
	}
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || (general.Tok != token.CONST && general.Tok != token.VAR) {
			continue
		}
		if value, found, err := findVersionValue(general, path, name); found || err != nil {
			return value, err
		}
	}
	return "", fmt.Errorf("%s constant not found in %s", name, path)
}

// findVersionValue looks for name in one const or var declaration; found is
// true when the identifier is declared there (valid or not).
func findVersionValue(general *ast.GenDecl, path, name string) (string, bool, error) {
	for _, rawSpec := range general.Specs {
		spec, ok := rawSpec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for index, identifier := range spec.Names {
			if identifier.Name != name {
				continue
			}
			value, err := versionLiteral(general, spec, index, path, name)
			return value, true, err
		}
	}
	return "", false, nil
}

// versionLiteral validates that the identifier at index is a constant string literal.
func versionLiteral(general *ast.GenDecl, spec *ast.ValueSpec, index int, path, name string) (string, error) {
	if general.Tok != token.CONST {
		return "", fmt.Errorf("%s in %s must be a constant, not a variable", name, path)
	}
	if index >= len(spec.Values) {
		return "", fmt.Errorf("%s in %s has no explicit value", name, path)
	}
	literal, ok := spec.Values[index].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", fmt.Errorf("%s in %s is not a string literal", name, path)
	}
	return strings.Trim(literal.Value, `"`), nil
}

func sortFindings(findings []finding) {
	sort.Slice(findings, func(left, right int) bool {
		if findings[left].path != findings[right].path {
			return findings[left].path < findings[right].path
		}
		if findings[left].line != findings[right].line {
			return findings[left].line < findings[right].line
		}
		return findings[left].message < findings[right].message
	})
}
