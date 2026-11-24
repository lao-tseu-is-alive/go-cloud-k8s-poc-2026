package legacyimport

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core/coretest"
	coremodule "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core/module"
)

// migratedTx opens a transaction on the migrated test database, rolled back
// at the end of the test.
func migratedTx(t *testing.T) (context.Context, pgx.Tx) {
	t.Helper()
	dsn := coretest.TestDatabaseURL(t, "legacy import schema tests")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := coremodule.Migrate(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.WithoutCancel(ctx)) })
	return ctx, tx
}

// indexDefinitions lists every index definition of tables.
func indexDefinitions(t *testing.T, ctx context.Context, tx pgx.Tx, tables []string) []string {
	t.Helper()
	rows, err := tx.Query(ctx, `SELECT pg_get_indexdef(indexrelid) FROM pg_index
		WHERE indrelid = ANY ($1::text[]::regclass[]) ORDER BY 1`, tables)
	if err != nil {
		t.Fatalf("list indexes: %v", err)
	}
	defs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("scan indexes: %v", err)
	}
	return defs
}

// TestBulkTablesExist keeps bulkTables in step with the schema: a renamed or
// dropped table would otherwise keep its indexes during the load unnoticed.
func TestBulkTablesExist(t *testing.T) {
	ctx, tx := migratedTx(t)
	if _, err := tx.Exec(ctx, `SELECT $1::text[]::regclass[]`, bulkTables); err != nil {
		t.Fatalf("a bulk table is missing from the schema: %v", err)
	}
}

// TestSecondaryIndexesAreRebuiltIdentical checks that the indexes set aside
// during the load come back exactly as they were and that the unique ones are
// never dropped. It works on temporary copies of three bulk tables with their
// real indexes (the full-text GIN ones included): DDL on the shared tables
// would lock out the other database tests running at the same time.
func TestSecondaryIndexesAreRebuiltIdentical(t *testing.T) {
	ctx, tx := migratedTx(t)
	copies := []string{"bulk_subject_ref", "bulk_subject_relationship", "bulk_document"}
	for i, source := range []string{"subject_ref", "subject_relationship", "document"} {
		sql := "CREATE TEMP TABLE " + copies[i] + " (LIKE " + source + " INCLUDING INDEXES) ON COMMIT DROP"
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	imp := &Importer{tx: tx, progress: newProgress(nil)}
	before := indexDefinitions(t, ctx, tx, copies)

	indexes, err := imp.dropSecondaryIndexes(ctx, copies)
	if err != nil {
		t.Fatalf("drop: %v", err)
	}
	if len(indexes) == 0 {
		t.Fatal("the bulk tables have secondary indexes to set aside")
	}
	during := indexDefinitions(t, ctx, tx, copies)
	for _, def := range during {
		for _, ix := range indexes {
			if def == ix.definition {
				t.Fatalf("index still present after the drop: %s", def)
			}
		}
	}
	if len(during)+len(indexes) != len(before) {
		t.Fatalf("%d indexes before, %d kept + %d dropped", len(before), len(during), len(indexes))
	}

	if err := imp.createIndexes(ctx, indexes); err != nil {
		t.Fatalf("create: %v", err)
	}
	if after := indexDefinitions(t, ctx, tx, copies); !slices.Equal(before, after) {
		t.Fatalf("indexes differ after the rebuild:\nbefore %v\nafter  %v", before, after)
	}
}

// insertTestDocumentSubjectSQL writes the subject of a test document.
const insertTestDocumentSubjectSQL = `INSERT INTO subject_ref (id, kind, display_label) VALUES ($1, 'DOCUMENT', 'Test')`

// statement is one SQL statement of a test with its arguments.
type statement struct {
	sql  string
	args []any
}

// execAll runs the statements in order and fails the test on the first error.
func execAll(t *testing.T, ctx context.Context, tx pgx.Tx, statements []statement) {
	t.Helper()
	for _, st := range statements {
		if _, err := tx.Exec(ctx, st.sql, st.args...); err != nil {
			t.Fatalf("%s: %v", st.sql, err)
		}
	}
}

// TestCurrentVersionReferenceCanBeDeferred checks migration 0038 as the
// documents stage uses it: a document written with its current version before
// the version, then the reference checked once the version is in. The stage
// must check before the indexes of document are built again: PostgreSQL
// refuses CREATE INDEX on a table with pending checks (SQLSTATE 55006).
func TestCurrentVersionReferenceCanBeDeferred(t *testing.T) {
	ctx, tx := migratedTx(t)
	var deferrable, deferred bool
	err := tx.QueryRow(ctx, `SELECT condeferrable, condeferred FROM pg_constraint
		WHERE conname = 'document_current_version_id_fkey'`).Scan(&deferrable, &deferred)
	if err != nil {
		t.Fatalf("read the constraint: %v", err)
	}
	if !deferrable || deferred {
		t.Fatalf("want deferrable and initially immediate, got deferrable=%v deferred=%v", deferrable, deferred)
	}

	doc, version := uuid.New(), uuid.New()
	execAll(t, ctx, tx, []statement{
		{deferCurrentVersionSQL, nil},
		{insertTestDocumentSubjectSQL, []any{doc}},
		{`INSERT INTO document (id, document_type_id, title, current_version_id)
		  SELECT $1, id, 'Test', $2 FROM document_type LIMIT 1`, []any{doc, version}},
		{`INSERT INTO document_version (id, document_id, version_no) VALUES ($1, $2, 1)`, []any{version, doc}},
		{checkCurrentVersionSQL, nil},
	})
}

// TestDanglingCurrentVersionIsRefused checks that the deferred reference is
// still enforced: a document pointing at a version never written fails.
func TestDanglingCurrentVersionIsRefused(t *testing.T) {
	ctx, tx := migratedTx(t)
	doc := uuid.New()
	execAll(t, ctx, tx, []statement{
		{deferCurrentVersionSQL, nil},
		{insertTestDocumentSubjectSQL, []any{doc}},
		{`INSERT INTO document (id, document_type_id, title, current_version_id)
		  SELECT $1, id, 'Test', gen_random_uuid() FROM document_type LIMIT 1`, []any{doc}},
	})
	if _, err := tx.Exec(ctx, checkCurrentVersionSQL); err == nil {
		t.Fatal("a current version that does not exist must be refused")
	}
}
