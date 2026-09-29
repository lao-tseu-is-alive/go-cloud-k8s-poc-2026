package timeline

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// PostgresRepository implements Repository with pgx, composing core primitives.
type PostgresRepository struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

// NewPostgresRepository builds a PostgresRepository from a connection pool. A
// nil logger falls back to slog.Default.
func NewPostgresRepository(pool *pgxpool.Pool, log *slog.Logger) (*PostgresRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("%w: PostgreSQL pool is required", core.ErrInvalidInput)
	}
	if log == nil {
		log = slog.Default()
	}
	return &PostgresRepository{pool: pool, log: log}, nil
}

// inTx runs fn in a transaction and commits it when fn succeeds.
func (r *PostgresRepository) inTx(ctx context.Context, op string, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin %s: %w", op, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s: %w", op, err)
	}
	return nil
}

// Create adds a draft to an open case: optional correction target checks, the
// entry, its cited documents (linked to the case when needed) and the
// TIMELINE_ENTRY_ADDED audit event in one transaction.
func (r *PostgresRepository) Create(ctx context.Context, in CreateInput) (*Entry, *core.AuditEvent, error) {
	var created *Entry
	var ev *core.AuditEvent
	err := r.inTx(ctx, "create timeline entry", func(tx pgx.Tx) error {
		if err := core.EnsureOpenCaseTx(ctx, tx, in.CaseID); err != nil {
			return err
		}
		if in.CorrectsEntryID != nil {
			if err := checkCorrectionTargetTx(ctx, tx, in.CaseID, *in.CorrectsEntryID); err != nil {
				return err
			}
		}
		e, err := collectEntry(tx.Query(ctx, insertEntrySQL, pgx.NamedArgs{
			"case_id":           in.CaseID,
			"entry_type":        int16(in.Type),
			"title":             in.Title,
			"body":              in.Body,
			"visibility":        int16(in.Visibility),
			"occurred_at":       in.OccurredAt,
			"corrects_entry_id": in.CorrectsEntryID,
			"created_by":        in.OperatorID,
		}))
		if err != nil {
			return err
		}
		documentIDs := make([]string, 0, len(in.DocumentIDs))
		for _, documentID := range in.DocumentIDs {
			if err := citeDocumentTx(ctx, tx, e, documentID, in.OperatorID); err != nil {
				return err
			}
			documentIDs = append(documentIDs, documentID.String())
		}
		after := entryState(e)
		after["document_ids"] = documentIDs
		created = e
		ev, err = auditTx(ctx, tx, e, "TIMELINE_ENTRY_ADDED", in.OperatorID, "", nil, after)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	e, err := r.Get(ctx, created.ID)
	return e, ev, err
}

// checkCorrectionTargetTx locks the entry to correct and requires it to belong
// to the same case, to be validated or locked and not to be corrected yet.
func checkCorrectionTargetTx(ctx context.Context, q core.Querier, caseID, targetID uuid.UUID) error {
	target, err := collectEntry(q.Query(ctx, getEntryForUpdateSQL, pgx.NamedArgs{"id": targetID}))
	if err != nil {
		return fmt.Errorf("corrected entry: %w", err)
	}
	if target.CaseID != caseID {
		return fmt.Errorf("%w: a correction must belong to the case of the corrected entry", core.ErrInvalidInput)
	}
	if !target.Status.Correctable() {
		return fmt.Errorf("%w: only a validated or locked entry is corrected; a %s entry is not", core.ErrInvalidState, target.Status)
	}
	var corrected bool
	if err := q.QueryRow(ctx, liveCorrectionExistsSQL, pgx.NamedArgs{"id": targetID}).Scan(&corrected); err != nil {
		return fmt.Errorf("check correction: %w", err)
	}
	if corrected {
		return fmt.Errorf("%w: the entry is already corrected; correct the correction instead", core.ErrConflict)
	}
	return nil
}

// Get loads an entry with its live document links.
func (r *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (*Entry, error) {
	e, err := collectEntry(r.pool.Query(ctx, getEntrySQL, pgx.NamedArgs{"id": id}))
	if err != nil {
		return nil, err
	}
	return e, r.hydrate(ctx, []*Entry{e})
}

// entryListRow adds the window total to the entry columns for list scanning.
type entryListRow struct {
	Entry
	// TotalSize is the COUNT(*) OVER () window total, repeated on every row.
	TotalSize int32 `db:"total_count"`
}

// List returns a page of a case timeline with the documents of each entry and
// the number of drafts of the case.
// An unknown case is core.ErrNotFound.
func (r *PostgresRepository) List(ctx context.Context, filter ListFilter) (ListResult, error) {
	if _, err := core.GetRecordMetadataTx(ctx, r.pool, filter.CaseID); err != nil {
		return ListResult{}, err
	}
	types := make([]int16, 0, len(filter.Types))
	for _, t := range filter.Types {
		types = append(types, int16(t))
	}
	rows, err := r.pool.Query(ctx, listEntriesSQL, pgx.NamedArgs{
		"case_id":           filter.CaseID,
		"entry_types":       types,
		"include_withdrawn": filter.IncludeWithdrawn,
		"limit":             filter.Limit,
		"offset":            filter.Offset,
	})
	if err != nil {
		return ListResult{}, fmt.Errorf("list timeline entries: %w", err)
	}
	listRows, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[entryListRow])
	if err != nil {
		return ListResult{}, fmt.Errorf("read timeline entries: %w", err)
	}
	drafts, err := countDraftsTx(ctx, r.pool, filter.CaseID)
	if err != nil {
		return ListResult{}, err
	}
	result := ListResult{Entries: make([]*Entry, len(listRows)), DraftCount: drafts}
	for i := range listRows {
		e := listRows[i].Entry
		result.Entries[i] = &e
		result.TotalSize = listRows[i].TotalSize
	}
	return result, r.hydrate(ctx, result.Entries)
}

// Update replaces the content of a draft and writes TIMELINE_ENTRY_UPDATED.
func (r *PostgresRepository) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Entry, *core.AuditEvent, error) {
	var ev *core.AuditEvent
	err := r.inTx(ctx, "update timeline entry", func(tx pgx.Tx) error {
		current, err := lockDraftTx(ctx, tx, id)
		if err != nil {
			return err
		}
		e, err := collectEntry(tx.Query(ctx, updateEntrySQL, pgx.NamedArgs{
			"id":          id,
			"entry_type":  int16(in.Type),
			"title":       in.Title,
			"body":        in.Body,
			"visibility":  int16(in.Visibility),
			"occurred_at": in.OccurredAt,
			"operator_id": in.OperatorID,
		}))
		if err != nil {
			return err
		}
		ev, err = auditTx(ctx, tx, e, "TIMELINE_ENTRY_UPDATED", in.OperatorID, in.Reason, entryState(current), entryState(e))
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	e, err := r.Get(ctx, id)
	return e, ev, err
}

// Validate endorses a draft (see freeze).
func (r *PostgresRepository) Validate(ctx context.Context, id uuid.UUID, operatorID, reason string) (*Entry, *core.AuditEvent, error) {
	return r.freeze(ctx, id, validateEntrySQL, "TIMELINE_ENTRY_VALIDATED", operatorID, reason)
}

// Lock freezes a draft as is (see freeze).
func (r *PostgresRepository) Lock(ctx context.Context, id uuid.UUID, operatorID, reason string) (*Entry, *core.AuditEvent, error) {
	return r.freeze(ctx, id, lockEntrySQL, "TIMELINE_ENTRY_LOCKED", operatorID, reason)
}

// freeze pins the current version of every cited document while the entry is
// still a draft, then moves it to its immutable status with statusSQL and
// writes eventType, in one transaction.
func (r *PostgresRepository) freeze(ctx context.Context, id uuid.UUID, statusSQL, eventType, operatorID, reason string) (*Entry, *core.AuditEvent, error) {
	var ev *core.AuditEvent
	err := r.inTx(ctx, "freeze timeline entry", func(tx pgx.Tx) error {
		current, err := lockDraftTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, pinVersionsSQL, pgx.NamedArgs{"entry_id": id}); err != nil {
			return fmt.Errorf("pin document versions: %w", err)
		}
		e, err := collectEntry(tx.Query(ctx, statusSQL, pgx.NamedArgs{"id": id, "operator_id": operatorID}))
		if err != nil {
			return err
		}
		ev, err = auditTx(ctx, tx, e, eventType, operatorID, reason,
			map[string]any{"status": current.Status.String()}, map[string]any{"status": e.Status.String()})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	e, err := r.Get(ctx, id)
	return e, ev, err
}

// Withdraw sets a draft aside with a reason and writes TIMELINE_ENTRY_WITHDRAWN.
func (r *PostgresRepository) Withdraw(ctx context.Context, id uuid.UUID, operatorID, reason string) (*Entry, *core.AuditEvent, error) {
	var ev *core.AuditEvent
	err := r.inTx(ctx, "withdraw timeline entry", func(tx pgx.Tx) error {
		current, err := lockDraftTx(ctx, tx, id)
		if err != nil {
			return err
		}
		e, err := collectEntry(tx.Query(ctx, withdrawEntrySQL, pgx.NamedArgs{"id": id, "operator_id": operatorID, "reason": reason}))
		if err != nil {
			return err
		}
		ev, err = auditTx(ctx, tx, e, "TIMELINE_ENTRY_WITHDRAWN", operatorID, reason,
			map[string]any{"status": current.Status.String()}, map[string]any{"status": e.Status.String()})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	e, err := r.Get(ctx, id)
	return e, ev, err
}

// LinkDocument cites a document in a draft and writes TIMELINE_DOCUMENT_LINKED.
func (r *PostgresRepository) LinkDocument(ctx context.Context, entryID, documentID uuid.UUID, operatorID string) (*Entry, *core.AuditEvent, error) {
	var ev *core.AuditEvent
	err := r.inTx(ctx, "link timeline document", func(tx pgx.Tx) error {
		e, err := lockDraftTx(ctx, tx, entryID)
		if err != nil {
			return err
		}
		if err := citeDocumentTx(ctx, tx, e, documentID, operatorID); err != nil {
			return err
		}
		ev, err = auditTx(ctx, tx, e, "TIMELINE_DOCUMENT_LINKED", operatorID, "", nil,
			map[string]any{"document_id": documentID.String()})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	e, err := r.Get(ctx, entryID)
	return e, ev, err
}

// UnlinkDocument removes a cited document from a draft (the link is kept as
// history) and writes TIMELINE_DOCUMENT_UNLINKED; the case relationship stays.
func (r *PostgresRepository) UnlinkDocument(ctx context.Context, entryID, documentID uuid.UUID, operatorID, reason string) (*Entry, *core.AuditEvent, error) {
	var ev *core.AuditEvent
	err := r.inTx(ctx, "unlink timeline document", func(tx pgx.Tx) error {
		e, err := lockDraftTx(ctx, tx, entryID)
		if err != nil {
			return err
		}
		var linkID uuid.UUID
		err = tx.QueryRow(ctx, removeLinkSQL, pgx.NamedArgs{"entry_id": entryID, "document_id": documentID, "operator_id": operatorID}).Scan(&linkID)
		if err != nil {
			return fmt.Errorf("document not cited by the entry: %w", mapDBError(err))
		}
		ev, err = auditTx(ctx, tx, e, "TIMELINE_DOCUMENT_UNLINKED", operatorID, reason,
			map[string]any{"document_id": documentID.String()}, nil)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	e, err := r.Get(ctx, entryID)
	return e, ev, err
}

// hydrate loads the live document links of entries in one query.
func (r *PostgresRepository) hydrate(ctx context.Context, entries []*Entry) error {
	if len(entries) == 0 {
		return nil
	}
	byID := make(map[uuid.UUID]*Entry, len(entries))
	ids := make([]uuid.UUID, 0, len(entries))
	for _, e := range entries {
		byID[e.ID] = e
		ids = append(ids, e.ID)
	}
	rows, err := r.pool.Query(ctx, listLinksSQL, pgx.NamedArgs{"entry_ids": ids})
	if err != nil {
		return fmt.Errorf("list timeline documents: %w", err)
	}
	links, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[DocumentLink])
	if err != nil {
		return fmt.Errorf("read timeline documents: %w", err)
	}
	for _, link := range links {
		e := byID[link.EntryID]
		e.Documents = append(e.Documents, link)
	}
	return nil
}
