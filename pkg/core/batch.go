package core

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Batch loading for list pages: one query per kind of related row for a whole
// page instead of one per row. Every query takes its ids as the @ids uuid[]
// argument (`WHERE col = ANY(@ids::uuid[])`).

// UniqueIDs returns ids without duplicates, in first-seen order.
func UniqueIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; !dup {
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

// collectByIDs runs sql with @ids and scans every row as T by name. No ids runs
// no query.
func collectByIDs[T any](ctx context.Context, q Querier, sql string, ids []uuid.UUID) ([]*T, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := q.Query(ctx, sql, pgx.NamedArgs{"ids": UniqueIDs(ids)})
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[T])
}

// CollectIndexedTx loads the rows of sql for ids and indexes them by key; every
// id must match a row, otherwise it returns ErrNotFound (a list page never shows
// a row whose parent reference is missing).
func CollectIndexedTx[T any](ctx context.Context, q Querier, sql string, ids []uuid.UUID, key func(*T) uuid.UUID) (map[uuid.UUID]*T, error) {
	rows, err := collectByIDs[T](ctx, q, sql, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*T, len(rows))
	for _, row := range rows {
		byID[key(row)] = row
	}
	for _, id := range ids {
		if _, ok := byID[id]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
		}
	}
	return byID, nil
}

// CollectGroupedTx loads the rows of sql for ids and groups them by key, in the
// query's order; an id without rows maps to nothing.
func CollectGroupedTx[T any](ctx context.Context, q Querier, sql string, ids []uuid.UUID, key func(*T) uuid.UUID) (map[uuid.UUID][]*T, error) {
	rows, err := collectByIDs[T](ctx, q, sql, ids)
	if err != nil {
		return nil, err
	}
	grouped := make(map[uuid.UUID][]*T, len(ids))
	for _, row := range rows {
		k := key(row)
		grouped[k] = append(grouped[k], row)
	}
	return grouped, nil
}

// SubjectHeaders are the identities and governance records of a page of
// subjects, indexed by subject id.
type SubjectHeaders struct {
	// Refs are the subject_ref rows.
	Refs map[uuid.UUID]*SubjectRef
	// Metadata are the record_metadata rows.
	Metadata map[uuid.UUID]*RecordMetadata
}

// GetSubjectHeadersTx loads the subject_ref and record_metadata of ids in two
// queries. A missing subject is ErrNotFound.
func GetSubjectHeadersTx(ctx context.Context, q Querier, ids []uuid.UUID) (SubjectHeaders, error) {
	refs, err := CollectIndexedTx(ctx, q, getSubjectRefsByIDsSQL, ids, func(r *SubjectRef) uuid.UUID { return r.ID })
	if err != nil {
		return SubjectHeaders{}, fmt.Errorf("load subjects: %w", err)
	}
	mds, err := CollectIndexedTx(ctx, q, getRecordMetadataByIDsSQL, ids, func(m *RecordMetadata) uuid.UUID { return m.SubjectID })
	if err != nil {
		return SubjectHeaders{}, fmt.Errorf("load governance: %w", err)
	}
	return SubjectHeaders{Refs: refs, Metadata: mds}, nil
}

// IDsOf maps items to their ids, for the batch loaders.
func IDsOf[T any](items []T, id func(T) uuid.UUID) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		ids = append(ids, id(item))
	}
	return slices.Clip(ids)
}
