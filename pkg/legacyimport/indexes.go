package legacyimport

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// bulkTables are the tables a run fills with hundreds of thousands of rows or
// more. Their secondary indexes are dropped before the load and created again
// once every row is in: one index build over the finished table is far
// cheaper than millions of random insertions into it (measured 2026-11-24:
// the COPY rates fell from ~100k to ~10k rows/s as those indexes outgrew the
// cache).
var bulkTables = []string{
	"subject_ref", "record_metadata", "subject_provenance", "subject_relationship",
	"case_file", "access_grant", "actor", "actor_contact", "thing",
	"document", "document_version", "content_blob",
	"case_timeline_entry", "timeline_document_link",
}

// secondaryIndexesSQL lists the indexes of the tables that are neither unique
// nor behind a constraint: primary keys, unique indexes and exclusion
// constraints stay, so every invariant is still checked row by row and the
// foreign keys keep their referenced index. The definition comes from the
// catalogue, so the index built again is the one the migrations created.
const secondaryIndexesSQL = `
SELECT i.indexrelid::regclass::text, pg_get_indexdef(i.indexrelid)
FROM pg_index i
WHERE i.indrelid = ANY ($1::text[]::regclass[])
  AND NOT i.indisunique AND NOT i.indisprimary
  AND NOT EXISTS (SELECT 1 FROM pg_constraint c WHERE c.conindid = i.indexrelid)
ORDER BY 1;`

// secondaryIndex is an index set aside during the load.
type secondaryIndex struct {
	name       string
	definition string
}

// dropSecondaryIndexes drops the secondary indexes of tables (the import
// passes bulkTables) inside the import transaction (a dry run rolls the drop
// back with the rest) and returns them to be created again by createIndexes.
func (imp *Importer) dropSecondaryIndexes(ctx context.Context, tables []string) ([]secondaryIndex, error) {
	done := imp.progress.step("drop secondary indexes")
	rows, err := imp.tx.Query(ctx, secondaryIndexesSQL, tables)
	if err != nil {
		return nil, fmt.Errorf("list secondary indexes: %w", err)
	}
	indexes, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (secondaryIndex, error) {
		var ix secondaryIndex
		err := row.Scan(&ix.name, &ix.definition)
		return ix, err
	})
	if err != nil {
		return nil, fmt.Errorf("list secondary indexes: %w", err)
	}
	for _, ix := range indexes {
		if _, err := imp.tx.Exec(ctx, "DROP INDEX "+ix.name); err != nil {
			return nil, fmt.Errorf("drop index %s: %w", ix.name, err)
		}
	}
	done("indexes", len(indexes))
	return indexes, nil
}

// createIndexes builds the indexes set aside by dropSecondaryIndexes again.
func (imp *Importer) createIndexes(ctx context.Context, indexes []secondaryIndex) error {
	done := imp.progress.step("create secondary indexes", "indexes", len(indexes))
	for _, ix := range indexes {
		built := imp.progress.step("create index " + ix.name)
		if _, err := imp.tx.Exec(ctx, ix.definition); err != nil {
			return fmt.Errorf("create index %s: %w", ix.name, err)
		}
		built()
	}
	done()
	return nil
}
