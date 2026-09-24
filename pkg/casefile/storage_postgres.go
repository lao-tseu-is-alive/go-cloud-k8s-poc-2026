package casefile

import (
	"context"
	"errors"
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

// Create opens a case: subject_ref, record_metadata, business reference (explicit,
// or allocated in the case type's namespace), case row and CASE_CREATED audit
// event in one transaction.
func (r *PostgresRepository) Create(ctx context.Context, in CreateInput) (*Case, *core.AuditEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin create case: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	caseType, err := getCaseType(ctx, tx, getCaseTypeByCodeSQL, pgx.NamedArgs{"code": in.CaseTypeCode})
	if err != nil {
		return nil, nil, err
	}
	if !caseType.IsActive {
		return nil, nil, fmt.Errorf("%w: case type %q is inactive", core.ErrInvalidInput, in.CaseTypeCode)
	}
	ref, err := core.InsertSubjectRefTx(ctx, tx, core.SubjectKindCase, in.Title, "")
	if err != nil {
		return nil, nil, fmt.Errorf("insert subject_ref: %w", err)
	}
	if _, err := core.InsertRecordMetadataTx(ctx, tx, in.Governance, ref.ID); err != nil {
		return nil, nil, fmt.Errorf("insert record_metadata: %w", err)
	}
	if req := businessRefFor(in.BusinessRef, caseType); !req.IsZero() {
		if ref, err = core.AssignBusinessRefTx(ctx, tx, ref.ID, req); err != nil {
			return nil, nil, err
		}
	}
	c, err := collectCase(tx.Query(ctx, insertCaseSQL, pgx.NamedArgs{
		"id":           ref.ID,
		"case_type_id": caseType.ID,
		"title":        in.Title,
		"description":  in.Description,
		"metadata":     jsonMap(in.Metadata),
		"created_by":   in.OperatorID,
	}))
	if err != nil {
		return nil, nil, err
	}
	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   ref.ID,
		EventType:   "CASE_CREATED",
		ActorUserID: in.OperatorID,
		AfterState: map[string]any{
			"title":                  c.Title,
			"case_type":              caseType.Code,
			"business_ref":           ref.BusinessRef,
			"business_ref_namespace": ref.BusinessRefNamespace,
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("insert audit_event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit create case: %w", err)
	}
	return c, ev, r.hydrate(ctx, c)
}

// businessRefFor returns the explicit request, or an allocation in the case
// type's namespace when the request is empty and the type has one.
func businessRefFor(req core.BusinessRefRequest, caseType *CaseType) core.BusinessRefRequest {
	if req.IsZero() && caseType.BusinessRefNamespace != "" {
		return core.BusinessRefRequest{Namespace: caseType.BusinessRefNamespace, Allocate: true}
	}
	return req
}

// Get loads a case with its subject, governance and type hydrated.
func (r *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (*Case, error) {
	c, err := collectCase(r.pool.Query(ctx, getCaseSQL, pgx.NamedArgs{"id": id}))
	if err != nil {
		return nil, err
	}
	return c, r.hydrate(ctx, c)
}

// Update replaces the editable metadata of an open (not closed), unlocked,
// live case, keeps the subject label in sync and writes CASE_UPDATED.
func (r *PostgresRepository) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Case, *core.AuditEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin update case: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := lockMutableCase(ctx, tx, id)
	if err != nil {
		return nil, nil, err
	}
	if current.Status == StatusClosed {
		return nil, nil, fmt.Errorf("%w: a closed case must be reopened before it is edited", core.ErrInvalidState)
	}
	c, err := collectCase(tx.Query(ctx, updateCaseSQL, pgx.NamedArgs{
		"id":          id,
		"title":       in.Title,
		"description": in.Description,
		"metadata":    jsonMap(in.Metadata),
	}))
	if err != nil {
		return nil, nil, err
	}
	if err := core.UpdateSubjectLabelTx(ctx, tx, id, c.Title); err != nil {
		return nil, nil, fmt.Errorf("sync subject label: %w", err)
	}
	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   id,
		EventType:   "CASE_UPDATED",
		ActorUserID: in.OperatorID,
		Reason:      in.Reason,
		BeforeState: map[string]any{"title": current.Title},
		AfterState:  map[string]any{"title": c.Title},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("insert audit_event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit update case: %w", err)
	}
	return c, ev, r.hydrate(ctx, c)
}

// Transition moves an unlocked, live case to in.Target when CanTransition
// allows it, and writes CASE_STATUS_CHANGED with the before/after status.
func (r *PostgresRepository) Transition(ctx context.Context, id uuid.UUID, in TransitionInput) (*Case, *core.AuditEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin transition case: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := lockMutableCase(ctx, tx, id)
	if err != nil {
		return nil, nil, err
	}
	if !CanTransition(current.Status, in.Target) {
		return nil, nil, fmt.Errorf("%w: cannot move a case from %s to %s", core.ErrInvalidState, current.Status, in.Target)
	}
	if RequiresReason(current.Status, in.Target) && in.Reason == "" {
		return nil, nil, fmt.Errorf("%w: a reason is required to close or reopen a case", core.ErrInvalidInput)
	}
	c, err := collectCase(tx.Query(ctx, transitionCaseSQL, pgx.NamedArgs{
		"id":          id,
		"status":      int16(in.Target),
		"operator_id": in.OperatorID,
		"reason":      in.Reason,
	}))
	if err != nil {
		return nil, nil, err
	}
	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   id,
		EventType:   "CASE_STATUS_CHANGED",
		ActorUserID: in.OperatorID,
		Reason:      in.Reason,
		BeforeState: map[string]any{"status": current.Status.String()},
		AfterState:  map[string]any{"status": c.Status.String()},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("insert audit_event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit transition case: %w", err)
	}
	return c, ev, r.hydrate(ctx, c)
}

// lockMutableCase rejects a locked or soft-deleted case (core.EnsureMutableTx)
// and returns the case row locked for the rest of the transaction.
func lockMutableCase(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*Case, error) {
	if _, err := core.EnsureMutableTx(ctx, tx, id, false); err != nil {
		return nil, err
	}
	return collectCase(tx.Query(ctx, getCaseForUpdateSQL, pgx.NamedArgs{"id": id}))
}

// SoftDelete logically deletes the case via its governance record and writes CASE_DELETED.
func (r *PostgresRepository) SoftDelete(ctx context.Context, id uuid.UUID, operatorID, reason string) (*core.AuditEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin delete case: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// An already soft-deleted case is rejected; a locked one may be retired.
	if _, err := core.EnsureMutableTx(ctx, tx, id, true); err != nil {
		return nil, err
	}
	if _, err := core.SoftDeleteRecordMetadataTx(ctx, tx, id, operatorID); err != nil {
		return nil, err
	}
	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   id,
		EventType:   "CASE_DELETED",
		ActorUserID: operatorID,
		Reason:      reason,
	})
	if err != nil {
		return nil, fmt.Errorf("insert audit_event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit delete case: %w", err)
	}
	return ev, nil
}

// caseListRow adds the window total to the case columns for search scanning.
type caseListRow struct {
	Case
	// TotalSize is the COUNT(*) OVER () window total, repeated on every row.
	TotalSize int32 `db:"total_count"`
}

// Search runs the filtered search and hydrates the results.
func (r *PostgresRepository) Search(ctx context.Context, filter SearchFilter) (SearchResult, error) {
	rows, err := r.pool.Query(ctx, searchCasesSQL, pgx.NamedArgs{
		"query":           filter.Query,
		"case_type_code":  filter.CaseTypeCode,
		"status":          int16(filter.Status),
		"include_deleted": filter.IncludeDeleted,
		"limit":           filter.Limit,
		"offset":          filter.Offset,
	})
	if err != nil {
		return SearchResult{}, fmt.Errorf("search cases: %w", err)
	}
	listRows, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[caseListRow])
	if err != nil {
		return SearchResult{}, fmt.Errorf("read cases: %w", err)
	}
	result := SearchResult{Cases: make([]*Case, len(listRows))}
	for i := range listRows {
		c := listRows[i].Case
		result.Cases[i] = &c
		result.TotalSize = listRows[i].TotalSize
		if err := r.hydrate(ctx, &c); err != nil {
			return SearchResult{}, err
		}
	}
	return result, nil
}

// ListTypes returns the case type catalogue ordered by code.
func (r *PostgresRepository) ListTypes(ctx context.Context, onlyActive bool) ([]*CaseType, error) {
	rows, err := r.pool.Query(ctx, listCaseTypesSQL, pgx.NamedArgs{"only_active": onlyActive})
	if err != nil {
		return nil, fmt.Errorf("list case types: %w", err)
	}
	types, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[CaseType])
	if err != nil {
		return nil, fmt.Errorf("read case types: %w", err)
	}
	return types, nil
}

// hydrate fills Subject, RecordMetadata and Type on a case.
func (r *PostgresRepository) hydrate(ctx context.Context, c *Case) error {
	ref, err := core.GetSubjectRefTx(ctx, r.pool, c.ID)
	if err != nil {
		return fmt.Errorf("hydrate subject: %w", err)
	}
	md, err := core.GetRecordMetadataTx(ctx, r.pool, c.ID)
	if err != nil {
		return fmt.Errorf("hydrate metadata: %w", err)
	}
	caseType, err := getCaseType(ctx, r.pool, getCaseTypeByIDSQL, pgx.NamedArgs{"id": c.CaseTypeID})
	if err != nil {
		return fmt.Errorf("hydrate type: %w", err)
	}
	c.Subject, c.RecordMetadata, c.Type = ref, md, caseType
	return nil
}

// collectCase reads exactly one case row from a query result.
func collectCase(rows pgx.Rows, err error) (*Case, error) {
	if err != nil {
		return nil, fmt.Errorf("query case: %w", err)
	}
	c, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Case])
	if err != nil {
		return nil, mapDBError(err)
	}
	return c, nil
}

// getCaseType loads one case type with the given query and arguments.
func getCaseType(ctx context.Context, q core.Querier, sql string, args pgx.NamedArgs) (*CaseType, error) {
	rows, err := q.Query(ctx, sql, args)
	if err != nil {
		return nil, fmt.Errorf("query case type: %w", err)
	}
	t, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[CaseType])
	if err != nil {
		return nil, mapDBError(err)
	}
	return t, nil
}

// jsonMap turns a nil map into an empty JSON object.
func jsonMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

// mapDBError translates pgx.ErrNoRows to core.ErrNotFound.
func mapDBError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return core.ErrNotFound
	}
	return err
}
