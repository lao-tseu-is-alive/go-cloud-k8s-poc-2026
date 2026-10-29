package module

import (
	"context"
	"fmt"
	"io/fs"
	"regexp"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Separation of the schema owner and the runtime role (GLD-046). The schema
// owner applies the migrations (goeland-server migrate); the server then runs
// as a role limited to reading and writing rows. Not owning the tables, that
// role can neither alter the schema nor drop or disable the triggers that keep
// the audit, the validated timeline and the document versions immutable.

// RuntimeDeletableTables are the only tables the runtime role may DELETE from:
// the replaced default grants of a case type and the purged outbox events.
// Every other record is non-destructive (soft deletes); a new DELETE in the
// application code must be listed here (a unit test checks the sources).
var RuntimeDeletableTables = []string{"case_type_default_grant", "outbox_event"}

// roleNamePattern accepts a plain PostgreSQL role name.
var roleNamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// ValidRoleName reports whether role is a plain lower-case PostgreSQL identifier.
func ValidRoleName(role string) bool { return roleNamePattern.MatchString(role) }

// GrantRuntimeRole gives role exactly what the server needs on the current
// schema: read, insert and update every table, delete only from
// RuntimeDeletableTables, use the sequences; schema_migrations stays read-only.
// It runs as the schema owner after the migrations (tables created by a later
// migration need it again), in one transaction, and is idempotent. role must
// already exist (created by the database administrator, LOGIN, nothing else).
func GrantRuntimeRole(ctx context.Context, pool *pgxpool.Pool, role string) error {
	if !ValidRoleName(role) {
		return fmt.Errorf("core module: invalid runtime role name %q", role)
	}
	var exists, self bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1), $1 = current_user`, role).Scan(&exists, &self); err != nil {
		return fmt.Errorf("core module: look up runtime role: %w", err)
	}
	if !exists {
		return fmt.Errorf("core module: runtime role %q does not exist; create it first (LOGIN, no other privilege)", role)
	}
	if self {
		return fmt.Errorf("core module: the runtime role %q must differ from the schema owner running the migrations", role)
	}
	r := pgx.Identifier{role}.Sanitize()
	statements := []string{
		"GRANT USAGE ON SCHEMA public TO " + r,
		"REVOKE ALL ON ALL TABLES IN SCHEMA public FROM " + r,
		"GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA public TO " + r,
		"REVOKE INSERT, UPDATE ON schema_migrations FROM " + r,
		"GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO " + r,
	}
	for _, table := range RuntimeDeletableTables {
		statements = append(statements, "GRANT DELETE ON "+pgx.Identifier{table}.Sanitize()+" TO "+r)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("core module: begin runtime grants: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement); err != nil {
			return fmt.Errorf("core module: grant runtime role: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("core module: commit runtime grants: %w", err)
	}
	return nil
}

// PendingMigrations returns the versions of the embedded migrations not yet
// applied to the database, without changing anything: a server that does not
// migrate (GOELAND_DB_MIGRATE=verify) refuses to start while any is pending.
func PendingMigrations(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	embedded, err := embeddedVersions()
	if err != nil {
		return nil, err
	}
	var present bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('schema_migrations') IS NOT NULL`).Scan(&present); err != nil {
		return nil, fmt.Errorf("core module: look up schema_migrations: %w", err)
	}
	if !present {
		return embedded, nil
	}
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("core module: read schema_migrations: %w", err)
	}
	applied, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("core module: read schema_migrations: %w", err)
	}
	return pendingVersions(embedded, applied), nil
}

// CanAlterSchema reports whether the connected role is a superuser or owns
// the audit log, and could therefore alter the schema or drop its triggers.
func CanAlterSchema(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	var can bool
	err := pool.QueryRow(ctx, `
SELECT r.rolsuper OR coalesce(pg_has_role(current_user, c.relowner, 'MEMBER'), false)
FROM pg_roles r LEFT JOIN pg_class c ON c.oid = to_regclass('audit_event')
WHERE r.rolname = current_user`).Scan(&can)
	if err != nil {
		return false, fmt.Errorf("core module: read runtime role privileges: %w", err)
	}
	return can, nil
}

// embeddedVersions lists the versions of the embedded migrations, in order.
func embeddedVersions() ([]string, error) {
	paths, err := fs.Glob(Migrations, "db/migrations/*.sql")
	if err != nil {
		return nil, fmt.Errorf("core module: list embedded migrations: %w", err)
	}
	sort.Strings(paths)
	versions := make([]string, 0, len(paths))
	for _, p := range paths {
		versions = append(versions, migrationVersion(p))
	}
	return versions, nil
}

// pendingVersions returns the embedded versions missing from applied, in order.
func pendingVersions(embedded, applied []string) []string {
	done := make(map[string]bool, len(applied))
	for _, v := range applied {
		done[v] = true
	}
	var pending []string
	for _, v := range embedded {
		if !done[v] {
			pending = append(pending, v)
		}
	}
	return pending
}
