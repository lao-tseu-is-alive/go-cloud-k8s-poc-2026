package core

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Access log (GLD-089, extending the sensitive read audit GLD-033, spec v2
// §34). The detail read of any subject, the download of any document's bytes
// and the export of a case, allowed or refused, and a refused change of a
// subject leave an access_audit_event row, written before the answer: when the
// row cannot be written the read fails (no trace, no read). A repeated access
// (same reader, subject, action, outcome and version) within AccessLogWindow is
// recorded once, so the page reloads of one visit make one consultation; an
// export is always recorded. Lists, searches and secondary reads are not
// logged; successful mutations stay in audit_event.

// AccessLogWindow is the period within which a repeated access is recorded once.
const AccessLogWindow = 15 * time.Minute

// AccessAction is what an access did; it mirrors access_audit_event.action.
type AccessAction string

// Access actions.
const (
	// AccessRead is the detail read of a subject (its Get RPC, or the
	// timeline of a case), whatever its confidentiality.
	AccessRead AccessAction = "READ"
	// AccessReadSensitive is the detail read of a confidential subject as
	// GLD-033 recorded it; no longer written (AccessRead), kept in the history
	// and matched by a filter on AccessRead.
	AccessReadSensitive AccessAction = "READ_SENSITIVE"
	// AccessDownload is the download of a document's bytes.
	AccessDownload AccessAction = "DOWNLOAD"
	// AccessExport is the export of a case and of each document it includes.
	AccessExport AccessAction = "EXPORT"
	// AccessWriteDenied is a change of a subject refused for lack of level
	// (always DENIED; a change allowed is in audit_event).
	AccessWriteDenied AccessAction = "WRITE_DENIED"
)

// AccessOutcome tells whether an access was allowed.
type AccessOutcome string

// Access outcomes.
const (
	// AccessGranted is an access the caller's level allowed.
	AccessGranted AccessOutcome = "GRANTED"
	// AccessDenied is an access refused for lack of level.
	AccessDenied AccessOutcome = "DENIED"
)

// AccessAuditEvent is one row of the access log.
type AccessAuditEvent struct {
	// ID is the server-assigned row id.
	ID uuid.UUID `db:"id"`
	// OccurredAt is when the access happened.
	OccurredAt time.Time `db:"occurred_at"`
	// UserID is the operator id of the reader.
	UserID string `db:"user_id"`
	// SubjectID is the subject read.
	SubjectID uuid.UUID `db:"subject_id"`
	// SubjectKind is the subject's kind.
	SubjectKind SubjectKind `db:"subject_kind"`
	// Action is what the access did.
	Action AccessAction `db:"action"`
	// Outcome tells whether it was allowed.
	Outcome AccessOutcome `db:"outcome"`
	// AccessLevel is the reader's effective level at that moment.
	AccessLevel Level `db:"access_level"`
	// AccessSource is the rule that produced AccessLevel.
	AccessSource AccessSource `db:"access_source"`
	// ConfidentialityLevel is the subject's confidentiality at that moment.
	ConfidentialityLevel int32 `db:"confidentiality_level"`
	// DocumentVersionID is the version downloaded (DOWNLOAD only).
	DocumentVersionID *uuid.UUID `db:"document_version_id"`
	// RequestID is the X-Request-ID of the request.
	RequestID string `db:"request_id"`
	// BusinessRef is the subject's business reference, when it has one.
	BusinessRef string `db:"business_ref"`
	// SubjectLabel is the subject's label, only when the lister may read the
	// subject (otherwise empty: the log never discloses a label).
	SubjectLabel string `db:"subject_label"`
}

// AccessAuditFilter selects a page of the access log.
type AccessAuditFilter struct {
	// Viewer is the lister: labels are shown only for subjects it may read.
	Viewer Viewer
	// SubjectID restricts the log to one subject; nil means every subject.
	SubjectID *uuid.UUID
	// UserID restricts the log to one reader; empty means any.
	UserID string
	// Action restricts the log to one action (AccessRead also matches the
	// former AccessReadSensitive); empty means any.
	Action AccessAction
	// Outcome restricts the log to one outcome; empty means any.
	Outcome AccessOutcome
	// From is the inclusive lower bound on OccurredAt; nil means unbounded.
	From *time.Time
	// To is the inclusive upper bound on OccurredAt; nil means unbounded.
	To *time.Time
	// Limit is the page size, normalized like RelationshipFilter.Limit.
	Limit int
	// Offset is the zero-based number of rows to skip.
	Offset int
	// OrderBy is the requested order ("<field> [asc|desc]"), parsed into Sort by the service.
	OrderBy string
	// Sort is the order of the log (accessAuditSortFields); the zero value is the default.
	Sort Sort
}

// AccessAuditResult is a page of the access log.
type AccessAuditResult struct {
	// Events is the requested page.
	Events []*AccessAuditEvent
	// TotalSize is the number of matching rows, at most MaxCountedTotal.
	TotalSize int32
	// TotalCapped reports that TotalSize is a lower bound.
	TotalCapped bool
}

// RequireAuditedTx requires need from access; a refusal is recorded (DENIED)
// before ErrPermissionDenied is returned.
func RequireAuditedTx(ctx context.Context, q Querier, userID string, access Access, need Level, action AccessAction) error {
	denied := access.Require(need)
	if denied == nil {
		return nil
	}
	if err := insertAccessAuditTx(ctx, q, userID, access, action, AccessDenied, nil); err != nil {
		return err
	}
	return denied
}

// RecordGrantedAccessTx records an allowed access (versionID names the
// downloaded version, nil otherwise).
func RecordGrantedAccessTx(ctx context.Context, q Querier, userID string, access Access, action AccessAction, versionID *uuid.UUID) error {
	return insertAccessAuditTx(ctx, q, userID, access, action, AccessGranted, versionID)
}

// RecordAccessTx records an access with its outcome (a case export, GLD-029).
func RecordAccessTx(ctx context.Context, q Querier, userID string, access Access, action AccessAction, outcome AccessOutcome) error {
	return insertAccessAuditTx(ctx, q, userID, access, action, outcome, nil)
}

// AuditedAccessTx is EnsureAccessTx for a logged access: it requires need on
// subjectID and records action, allowed or refused. It returns the caller's access.
func AuditedAccessTx(ctx context.Context, q Querier, userID string, subjectID uuid.UUID, need Level, action AccessAction) (Access, error) {
	access, err := EffectiveAccessTx(ctx, q, userID, subjectID)
	if err != nil {
		return Access{}, err
	}
	if err := RequireAuditedTx(ctx, q, userID, access, need, action); err != nil {
		return Access{}, err
	}
	return access, RecordGrantedAccessTx(ctx, q, userID, access, action, nil)
}

// RecordWriteDeniedTx records a change of a subject refused to userID.
func RecordWriteDeniedTx(ctx context.Context, q Querier, userID string, access Access) error {
	return insertAccessAuditTx(ctx, q, userID, access, AccessWriteDenied, AccessDenied, nil)
}

// insertAccessAuditTx writes one access_audit_event row, unless the same
// access was recorded within AccessLogWindow (an export always is).
func insertAccessAuditTx(ctx context.Context, q Querier, userID string, access Access, action AccessAction, outcome AccessOutcome, versionID *uuid.UUID) error {
	window := AccessLogWindow
	if action == AccessExport {
		window = 0
	}
	if _, err := q.Exec(ctx, insertAccessAuditSQL, pgx.NamedArgs{
		"user_id": userID, "subject_id": access.SubjectID, "subject_kind": string(access.Kind),
		"action": string(action), "outcome": string(outcome), "access_level": int16(access.Level),
		"access_source": string(access.Source), "confidentiality_level": access.Confidentiality,
		"document_version_id": versionID, "request_id": RequestIDFromContext(ctx), "window": window,
	}); err != nil {
		return fmt.Errorf("record access: %w", err)
	}
	return nil
}

// ListAccessAudit pages the access log; the caller has checked who may read it.
func (s *Service) ListAccessAudit(ctx context.Context, filter AccessAuditFilter) (AccessAuditResult, error) {
	limit, err := NormalizePageSize(filter.Limit)
	if err != nil {
		return AccessAuditResult{}, err
	}
	filter.Limit, filter.Offset = limit, max(filter.Offset, 0)
	if filter.Sort, err = ParseOrderBy(filter.OrderBy, SortNames(accessAuditSortFields), defaultAccessAuditSort); err != nil {
		return AccessAuditResult{}, err
	}
	return s.repo.ListAccessAudit(ctx, filter)
}

// ListAccessAudit returns a page of the access log.
func (r *PostgresRepository) ListAccessAudit(ctx context.Context, filter AccessAuditFilter) (AccessAuditResult, error) {
	rows, err := r.pool.Query(ctx, SortedQuery(listAccessAuditSQL, filter.Sort, defaultAccessAuditSort), filter.Viewer.AddTo(pgx.NamedArgs{
		"subject_id": filter.SubjectID, "user_id": filter.UserID, "action": string(filter.Action),
		"outcome": string(filter.Outcome), "from": filter.From, "to": filter.To,
		"count_limit": CountLimit, "limit": filter.Limit, "offset": filter.Offset,
	}))
	if err != nil {
		return AccessAuditResult{}, fmt.Errorf("list accesses: %w", err)
	}
	type row struct {
		AccessAuditEvent
		TotalCount int32 `db:"total_count"`
	}
	listed, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[row])
	if err != nil {
		return AccessAuditResult{}, fmt.Errorf("read accesses: %w", err)
	}
	result := AccessAuditResult{Events: make([]*AccessAuditEvent, len(listed))}
	for i := range listed {
		event := listed[i].AccessAuditEvent
		result.Events[i] = &event
		result.TotalSize = listed[i].TotalCount
	}
	result.TotalSize, result.TotalCapped = CapTotal(result.TotalSize)
	return result, nil
}
