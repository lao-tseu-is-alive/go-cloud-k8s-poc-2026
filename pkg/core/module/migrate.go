// Package module provides an importable, bundleable Module for the Goéland core domain.
package module

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrations holds the embedded SQL migration files for the whole Goéland POC schema.
//
// For this initial POC the core module owns the schema bootstrap (subject core,
// relationships, document, seed data) because the Document tables have foreign
// keys into the core tables. As the POC grows, migrations can be split per module.
//
//go:embed db/migrations/*.sql
var Migrations embed.FS

const migrationLockKey = "go-cloud-k8s-poc-2026:migrations"

// Migrate applies all pending SQL migrations. It takes a PostgreSQL advisory lock
// so only one instance migrates at a time (safe for rolling deployments).
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("core module: migration pool is required")
	}
	paths, err := fs.Glob(Migrations, "db/migrations/*.sql")
	if err != nil {
		return fmt.Errorf("core module: list embedded migrations: %w", err)
	}
	sort.Strings(paths)

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("core module: acquire migration connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock(hashtext($1))", migrationLockKey); err != nil {
		return fmt.Errorf("core module: acquire migration lock: %w", err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = conn.Exec(unlockCtx, "SELECT pg_advisory_unlock(hashtext($1))", migrationLockKey)
	}()

	if _, err := conn.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`); err != nil {
		return fmt.Errorf("core module: create schema_migrations table: %w", err)
	}
	for _, path := range paths {
		if err := migrateOne(ctx, conn.Conn(), path); err != nil {
			return err
		}
	}
	return nil
}

// migrateOne applies the embedded migration at path unless its version is
// already recorded in schema_migrations.
func migrateOne(ctx context.Context, conn *pgx.Conn, path string) error {
	version := migrationVersion(path)
	var applied bool
	if err := conn.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", version,
	).Scan(&applied); err != nil {
		return fmt.Errorf("core module: check migration %s: %w", version, err)
	}
	if applied {
		return nil
	}
	content, err := Migrations.ReadFile(path)
	if err != nil {
		return fmt.Errorf("core module: read migration %s: %w", version, err)
	}
	statements, err := ParseDBMateUp(string(content))
	if err != nil {
		return fmt.Errorf("core module: parse migration %s: %w", version, err)
	}
	return applyMigration(ctx, conn, version, statements)
}

// migrationVersion extracts the version prefix (before the first underscore) from a migration filename.
func migrationVersion(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	version, _, found := strings.Cut(base, "_")
	if found {
		return version
	}
	return base
}

// applyMigration runs the statements in a transaction and records the version.
func applyMigration(ctx context.Context, conn *pgx.Conn, version string, statements []string) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("core module: begin migration %s: %w", version, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement); err != nil {
			return fmt.Errorf("core module: apply migration %s: %w", version, err)
		}
	}
	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
		return fmt.Errorf("core module: record migration %s: %w", version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("core module: commit migration %s: %w", version, err)
	}
	return nil
}

// DBMate section and block markers understood by ParseDBMateUp.
const (
	upMarker             = "-- migrate:up"
	downMarker           = "-- migrate:down"
	statementBeginMarker = "-- migrate:statementbegin"
	statementEndMarker   = "-- migrate:statementend"
)

// ParseDBMateUp extracts the SQL statements from the "-- migrate:up" section of a
// DBMate migration file. Blocks delimited by "-- migrate:statementbegin/end"
// (e.g. PL/pgSQL functions) are emitted as a single statement.
func ParseDBMateUp(content string) ([]string, error) {
	_, rest, found := strings.Cut(content, upMarker)
	if !found {
		return nil, fmt.Errorf("missing %s marker", upMarker)
	}
	section, _, _ := strings.Cut(rest, downMarker)

	var p statementParser
	for line := range strings.SplitSeq(section, "\n") {
		if err := p.feed(line); err != nil {
			return nil, err
		}
	}
	if p.inBlock {
		return nil, fmt.Errorf("unterminated statement block")
	}
	p.flush()
	if len(p.statements) == 0 {
		return nil, fmt.Errorf("migration has no up statements")
	}
	return p.statements, nil
}

// statementParser accumulates lines into statements: a statement ends with a
// line ending in ";" outside a block, or with the end of a statement block.
type statementParser struct {
	statements []string
	current    strings.Builder
	inBlock    bool
}

// feed consumes one line of the up section.
func (p *statementParser) feed(line string) error {
	trimmed := strings.TrimSpace(line)
	switch trimmed {
	case statementBeginMarker:
		if p.inBlock {
			return fmt.Errorf("nested statement block")
		}
		p.flush()
		p.inBlock = true
		return nil
	case statementEndMarker:
		if !p.inBlock {
			return fmt.Errorf("statement end without begin")
		}
		p.flush()
		p.inBlock = false
		return nil
	}
	p.current.WriteString(line)
	p.current.WriteByte('\n')
	if !p.inBlock && strings.HasSuffix(trimmed, ";") {
		p.flush()
	}
	return nil
}

// flush emits the accumulated statement when it contains SQL.
func (p *statementParser) flush() {
	statement := strings.TrimSpace(p.current.String())
	p.current.Reset()
	if containsSQL(statement) {
		p.statements = append(p.statements, statement)
	}
}

// containsSQL reports whether the statement has at least one non-blank, non-comment line.
func containsSQL(statement string) bool {
	for line := range strings.SplitSeq(statement, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "--") {
			return true
		}
	}
	return false
}
