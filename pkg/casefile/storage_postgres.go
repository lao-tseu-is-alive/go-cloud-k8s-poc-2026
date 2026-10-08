package casefile

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/circulation"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/task"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/timeline"
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
	defaultGrants, err := applyTypeDefaultsTx(ctx, tx, caseType, &in, ref.ID)
	if err != nil {
		return nil, nil, err
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
			"confidentiality_level":  in.Governance.ConfidentialityLevel,
			"default_grants":         defaultGrants,
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

// applyTypeDefaultsTx writes the case's governance with the type's minimum
// confidentiality, then copies the type's default grants (GLD-050); it
// returns the grants written, for the creation event.
func applyTypeDefaultsTx(ctx context.Context, tx pgx.Tx, caseType *CaseType, in *CreateInput, caseID uuid.UUID) ([]map[string]any, error) {
	in.Governance.ConfidentialityLevel = max(in.Governance.ConfidentialityLevel, caseType.DefaultConfidentialityLevel)
	if _, err := core.InsertRecordMetadataTx(ctx, tx, in.Governance, caseID); err != nil {
		return nil, fmt.Errorf("insert record_metadata: %w", err)
	}
	if err := hydrateDefaultGrantsTx(ctx, tx, []*CaseType{caseType}); err != nil {
		return nil, err
	}
	return core.ApplyDefaultGrantsTx(ctx, tx, caseID, in.OperatorID, "default grant of case type "+caseType.Code, caseType.DefaultGrants)
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
// allows it, writes CASE_STATUS_CHANGED with the before/after status and
// records the change as a SYSTEM timeline entry. A case with draft timeline
// entries, open circulations or open tasks cannot be closed.
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
	if in.Target == StatusClosed {
		if err := ensureClosableTx(ctx, tx, id); err != nil {
			return nil, nil, err
		}
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
	if _, err := timeline.RecordSystemEntryTx(ctx, tx, statusChangeEntry(id, current.Status, c.Status, in)); err != nil {
		return nil, nil, fmt.Errorf("record timeline entry: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit transition case: %w", err)
	}
	return c, ev, r.hydrate(ctx, c)
}

// ensureClosableTx refuses to close a case that still has draft timeline
// entries, open circulations or open tasks (in that order of explanation).
func ensureClosableTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	if err := timeline.EnsureNoDraftsTx(ctx, tx, id); err != nil {
		return err
	}
	if err := circulation.EnsureNoOpenCirculationsTx(ctx, tx, id); err != nil {
		return err
	}
	return task.EnsureNoOpenTasksTx(ctx, tx, id)
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
	rows, err := r.pool.Query(ctx, core.SortedQuery(searchCasesSQL, filter.Sort, defaultCaseSort), filter.Viewer.AddTo(pgx.NamedArgs{
		"count_limit":     core.CountLimit,
		"query":           filter.Query,
		"case_type_code":  filter.CaseTypeCode,
		"status":          int16(filter.Status),
		"include_deleted": filter.IncludeDeleted,
		"limit":           filter.Limit,
		"offset":          filter.Offset,
	}))
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
	}
	result.TotalSize, result.TotalCapped = core.CapTotal(result.TotalSize)
	return result, r.hydrateAll(ctx, result.Cases)
}

// ListTypes returns the case type catalogue ordered by label, code as tie-break.
func (r *PostgresRepository) ListTypes(ctx context.Context, onlyActive bool) ([]*CaseType, error) {
	rows, err := r.pool.Query(ctx, listCaseTypesSQL, pgx.NamedArgs{"only_active": onlyActive})
	if err != nil {
		return nil, fmt.Errorf("list case types: %w", err)
	}
	types, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[CaseType])
	if err != nil {
		return nil, fmt.Errorf("read case types: %w", err)
	}
	return types, hydrateDefaultGrantsTx(ctx, r.pool, types)
}

// hydrate fills Subject, RecordMetadata and Type on a case.
func (r *PostgresRepository) hydrate(ctx context.Context, c *Case) error {
	return r.hydrateAll(ctx, []*Case{c})
}

// hydrateAll fills Subject, RecordMetadata and Type on a page of cases in three
// queries, whatever the page size.
func (r *PostgresRepository) hydrateAll(ctx context.Context, cases []*Case) error {
	if len(cases) == 0 {
		return nil
	}
	headers, err := core.GetSubjectHeadersTx(ctx, r.pool, core.IDsOf(cases, func(c *Case) uuid.UUID { return c.ID }))
	if err != nil {
		return fmt.Errorf("hydrate cases: %w", err)
	}
	types, err := core.CollectIndexedTx(ctx, r.pool, getCaseTypesByIDsSQL,
		core.IDsOf(cases, func(c *Case) uuid.UUID { return c.CaseTypeID }), func(t *CaseType) uuid.UUID { return t.ID })
	if err != nil {
		return fmt.Errorf("hydrate case types: %w", err)
	}
	for _, c := range cases {
		c.Subject, c.RecordMetadata, c.Type = headers.Refs[c.ID], headers.Metadata[c.ID], types[c.CaseTypeID]
	}
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
	return core.MapDBError(err, nil)
}

// statusChangeEntry describes a status change as a SYSTEM timeline entry: a
// French readable body and the structured event for clients.
func statusChangeEntry(caseID uuid.UUID, from, to Status, in TransitionInput) timeline.SystemEntry {
	body := fmt.Sprintf("Statut de l'affaire : %s → %s", statusLabelsFR[from], statusLabelsFR[to])
	if in.Reason != "" {
		body += "\nMotif : " + in.Reason
	}
	return timeline.SystemEntry{
		CaseID: caseID,
		Title:  "Changement de statut",
		Body:   body,
		Metadata: map[string]any{
			"event":  "CASE_STATUS_CHANGED",
			"from":   from.String(),
			"to":     to.String(),
			"reason": in.Reason,
		},
		OperatorID: in.OperatorID,
	}
}
