package timeline

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// caseStatusClosed is case_file.status for a closed case (casefile.StatusClosed;
// the value is repeated here because casefile imports this package).
const caseStatusClosed = 4

// EnsureNoDraftsTx fails with core.ErrInvalidState when the case still has
// draft entries. The case lifecycle calls it before closing a case: a draft
// must be validated, locked or withdrawn first, never frozen half-written.
func EnsureNoDraftsTx(ctx context.Context, q core.Querier, caseID uuid.UUID) error {
	drafts, err := countDraftsTx(ctx, q, caseID)
	if err != nil {
		return err
	}
	if drafts > 0 {
		return fmt.Errorf("%w: the case has %d draft timeline entries; validate, lock or withdraw them before closing it", core.ErrInvalidState, drafts)
	}
	return nil
}

// countDraftsTx counts the draft entries of a case.
func countDraftsTx(ctx context.Context, q core.Querier, caseID uuid.UUID) (int32, error) {
	var drafts int32
	if err := q.QueryRow(ctx, countDraftsSQL, pgx.NamedArgs{"case_id": caseID}).Scan(&drafts); err != nil {
		return 0, fmt.Errorf("count timeline drafts: %w", err)
	}
	return drafts, nil
}

// RecordSystemEntryTx records a server-written fact in a case timeline, born
// locked, and writes its TIMELINE_ENTRY_ADDED audit event on the case, in the
// caller's transaction. It does not check the case status: the caller is the
// server reacting to a case mutation it has already validated.
func RecordSystemEntryTx(ctx context.Context, q core.Querier, in SystemEntry) (*Entry, error) {
	if in.CaseID == uuid.Nil || in.Body == "" {
		return nil, fmt.Errorf("%w: a system entry needs a case and a body", core.ErrInvalidInput)
	}
	e, err := collectEntry(q.Query(ctx, insertSystemEntrySQL, pgx.NamedArgs{
		"case_id":     in.CaseID,
		"title":       in.Title,
		"body":        in.Body,
		"metadata":    jsonMap(in.Metadata),
		"operator_id": in.OperatorID,
	}))
	if err != nil {
		return nil, err
	}
	if _, err := auditTx(ctx, q, e, "TIMELINE_ENTRY_ADDED", in.OperatorID, "", nil, entryState(e)); err != nil {
		return nil, err
	}
	return e, nil
}

// lockOpenCaseTx locks the case's governance row (rejecting a locked or
// deleted case) and rejects a closed case. The governance lock serializes
// every timeline mutation with the case status changes, which take it too.
func lockOpenCaseTx(ctx context.Context, q core.Querier, caseID uuid.UUID) error {
	if _, err := core.EnsureMutableTx(ctx, q, caseID, false); err != nil {
		return err
	}
	var status int16
	if err := q.QueryRow(ctx, caseStatusSQL, pgx.NamedArgs{"id": caseID}).Scan(&status); err != nil {
		return mapDBError(err)
	}
	if status == caseStatusClosed {
		return fmt.Errorf("%w: a closed case must be reopened before its timeline changes", core.ErrInvalidState)
	}
	return nil
}

// lockDraftTx locks the entry's case (see lockOpenCaseTx), then the entry
// itself, and rejects an entry that is no longer a draft. The case is locked
// first, in the same order as entry creation, to avoid lock inversions.
func lockDraftTx(ctx context.Context, q core.Querier, id uuid.UUID) (*Entry, error) {
	current, err := collectEntry(q.Query(ctx, getEntrySQL, pgx.NamedArgs{"id": id}))
	if err != nil {
		return nil, err
	}
	if err := lockOpenCaseTx(ctx, q, current.CaseID); err != nil {
		return nil, err
	}
	e, err := collectEntry(q.Query(ctx, getEntryForUpdateSQL, pgx.NamedArgs{"id": id}))
	if err != nil {
		return nil, err
	}
	if e.Status != StatusDraft {
		return nil, fmt.Errorf("%w: a %s timeline entry is immutable; create a correction instead", core.ErrInvalidState, e.Status)
	}
	return e, nil
}

// citeDocumentTx links a document to a draft entry. The document must be a
// live DOCUMENT subject; when no open CASE_HAS_DOCUMENT edge links it to the
// case yet, one is created and audited (RELATIONSHIP_LINKED) in the same
// transaction. A document already cited by the entry is core.ErrConflict.
func citeDocumentTx(ctx context.Context, q core.Querier, e *Entry, documentID uuid.UUID, operatorID string) error {
	ref, err := core.GetSubjectRefTx(ctx, q, documentID)
	if err != nil {
		return err
	}
	if ref.Kind != core.SubjectKindDocument {
		return fmt.Errorf("%w: subject %s is a %s, not a document", core.ErrInvalidInput, documentID, ref.Kind)
	}
	if _, err := core.EnsureMutableTx(ctx, q, documentID, true); err != nil {
		return err
	}
	if err := ensureCaseHasDocumentTx(ctx, q, e.CaseID, documentID, operatorID); err != nil {
		return err
	}
	var linkID uuid.UUID
	err = q.QueryRow(ctx, insertLinkSQL, pgx.NamedArgs{
		"entry_id":    e.ID,
		"document_id": documentID,
		"operator_id": operatorID,
	}).Scan(&linkID)
	if isUniqueViolation(err) {
		return fmt.Errorf("%w: the document is already cited by this entry", core.ErrConflict)
	}
	return mapDBError(err)
}

// ensureCaseHasDocumentTx links the document to the case when no open
// CASE_HAS_DOCUMENT edge exists, auditing it as CoreService.LinkSubjects does.
func ensureCaseHasDocumentTx(ctx context.Context, q core.Querier, caseID, documentID uuid.UUID, operatorID string) error {
	var linked bool
	if err := q.QueryRow(ctx, caseHasDocumentSQL, pgx.NamedArgs{"case_id": caseID, "document_id": documentID}).Scan(&linked); err != nil {
		return fmt.Errorf("check case document: %w", err)
	}
	if linked {
		return nil
	}
	rel, err := core.LinkSubjectsTx(ctx, q, core.LinkInput{
		SourceSubjectID:      caseID,
		TargetSubjectID:      documentID,
		RelationshipTypeCode: "CASE_HAS_DOCUMENT",
		OperatorID:           operatorID,
	})
	if err != nil {
		return fmt.Errorf("link document to case: %w", err)
	}
	_, err = core.InsertAuditEventTx(ctx, q, core.AuditEvent{
		SubjectID:   caseID,
		EventType:   "RELATIONSHIP_LINKED",
		ActorUserID: operatorID,
		AfterState: map[string]any{
			"relationship_id": rel.ID.String(),
			"type":            "CASE_HAS_DOCUMENT",
			"target":          documentID.String(),
		},
		Metadata: map[string]any{"cause": "TIMELINE_DOCUMENT_LINKED"},
	})
	if err != nil {
		return fmt.Errorf("insert audit_event: %w", err)
	}
	return nil
}

// auditTx writes a timeline audit event on the entry's CASE subject, naming the
// entry in metadata so the case trail shows the whole timeline history.
func auditTx(ctx context.Context, q core.Querier, e *Entry, eventType, operatorID, reason string, before, after map[string]any) (*core.AuditEvent, error) {
	ev, err := core.InsertAuditEventTx(ctx, q, core.AuditEvent{
		SubjectID:   e.CaseID,
		EventType:   eventType,
		ActorUserID: operatorID,
		Reason:      reason,
		BeforeState: before,
		AfterState:  after,
		Metadata:    map[string]any{"timeline_entry_id": e.ID.String()},
	})
	if err != nil {
		return nil, fmt.Errorf("insert audit_event: %w", err)
	}
	return ev, nil
}

// entryState is the audited snapshot of an entry.
func entryState(e *Entry) map[string]any {
	state := map[string]any{
		"entry_type":  e.Type.String(),
		"status":      e.Status.String(),
		"title":       e.Title,
		"occurred_at": e.OccurredAt,
	}
	if e.CorrectsEntryID != nil {
		state["corrects_entry_id"] = e.CorrectsEntryID.String()
	}
	return state
}

// collectEntry reads exactly one entry row from a query result.
func collectEntry(rows pgx.Rows, err error) (*Entry, error) {
	if err != nil {
		return nil, fmt.Errorf("query timeline entry: %w", err)
	}
	e, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Entry])
	if err != nil {
		return nil, mapDBError(err)
	}
	return e, nil
}

// jsonMap turns a nil map into an empty JSON object.
func jsonMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

// isUniqueViolation reports whether err is a PostgreSQL unique violation.
func isUniqueViolation(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == "23505"
}

// mapDBError translates pgx.ErrNoRows and foreign-key violations to
// core.ErrNotFound and unique violations to core.ErrConflict.
func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return core.ErrNotFound
	}
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		switch pgErr.Code {
		case "23505": // unique_violation
			return fmt.Errorf("%w: %s", core.ErrConflict, pgErr.Message)
		case "23503": // foreign_key_violation
			return fmt.Errorf("%w: %s", core.ErrNotFound, pgErr.Message)
		}
	}
	return err
}
