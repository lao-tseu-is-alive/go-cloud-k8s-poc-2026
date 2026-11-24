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

// indexDefinitions lists every index definition of the bulk tables.
func indexDefinitions(t *testing.T, ctx context.Context, tx pgx.Tx) []string {
	t.Helper()
	rows, err := tx.Query(ctx, `SELECT pg_get_indexdef(indexrelid) FROM pg_index
		WHERE indrelid = ANY ($1::text[]::regclass[]) ORDER BY 1`, bulkTables)
	if err != nil {
		t.Fatalf("list indexes: %v", err)
	}
	defs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("scan indexes: %v", err)
	}
	return defs
}

// TestSecondaryIndexesAreRebuiltIdentical checks that the indexes set aside
// during the load come back exactly as the migrations created them, and that
// the unique ones are never dropped.
func TestSecondaryIndexesAreRebuiltIdentical(t *testing.T) {
	ctx, tx := migratedTx(t)
	imp := &Importer{tx: tx, progress: newProgress(nil)}
	before := indexDefinitions(t, ctx, tx)

	indexes, err := imp.dropSecondaryIndexes(ctx)
	if err != nil {
		t.Fatalf("drop: %v", err)
	}
	if len(indexes) == 0 {
		t.Fatal("the bulk tables have secondary indexes to set aside")
	}
	during := indexDefinitions(t, ctx, tx)
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
	if after := indexDefinitions(t, ctx, tx); !slices.Equal(before, after) {
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
// the version, the reference checked once the version is in, then the indexes
// of document built again (pending checks would forbid it).
func TestCurrentVersionReferenceCanBeDeferred(t *testing.T) {
	ctx, tx := migratedTx(t)
	imp := &Importer{tx: tx, progress: newProgress(nil)}
	var deferrable, deferred bool
	err := tx.QueryRow(ctx, `SELECT condeferrable, condeferred FROM pg_constraint
		WHERE conname = 'document_current_version_id_fkey'`).Scan(&deferrable, &deferred)
	if err != nil {
		t.Fatalf("read the constraint: %v", err)
	}
	if !deferrable || deferred {
		t.Fatalf("want deferrable and initially immediate, got deferrable=%v deferred=%v", deferrable, deferred)
	}
	indexes, err := imp.dropSecondaryIndexes(ctx)
	if err != nil {
		t.Fatalf("drop: %v", err)
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
	if err := imp.createIndexes(ctx, indexes); err != nil {
		t.Fatalf("create the indexes after the check: %v", err)
	}
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
