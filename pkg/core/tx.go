package core

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier is the subset of pgx used by the transaction-scoped helpers below.
// Both *pgxpool.Pool and pgx.Tx satisfy it, so the same helper works standalone
// or composed inside another domain's transaction (e.g. the Document repository
// creating a subject_ref + record_metadata + document + audit atomically).
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// InsertSubjectRefTx inserts a canonical subject_ref row using q (pool or tx).
func InsertSubjectRefTx(ctx context.Context, q Querier, kind SubjectKind, displayLabel, canonicalURL string) (*SubjectRef, error) {
	rows, err := q.Query(ctx, insertSubjectRefSQL, string(kind), displayLabel, canonicalURL)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[SubjectRef])
}

// InsertRecordMetadataTx inserts the 1:1 governance record for a subject using q.
func InsertRecordMetadataTx(ctx context.Context, q Querier, in CreateSubjectInput, subjectID uuid.UUID) (*RecordMetadata, error) {
	metadata := in.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	rows, err := q.Query(ctx, insertRecordMetadataSQL,
		subjectID, in.ActorUserID, in.OwnerUserID, in.OwnerOrgID,
		in.ConfidentialityLevel, in.RetentionUntil, in.SortFinal, metadata)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[RecordMetadata])
}

// InsertAuditEventTx appends an append-only audit event using q. Every domain
// mutation must call this within the same transaction as the mutation itself.
func InsertAuditEventTx(ctx context.Context, q Querier, ev AuditEvent) (*AuditEvent, error) {
	var correlation *uuid.UUID
	if ev.CorrelationID != nil && *ev.CorrelationID != uuid.Nil {
		correlation = ev.CorrelationID
	}
	metadata := ev.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	rows, err := q.Query(ctx, insertAuditEventSQL,
		ev.SubjectID, ev.EventType, ev.ActorUserID, ev.BeforeState, ev.AfterState,
		ev.Reason, correlation, ev.RequestID, metadata)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[AuditEvent])
}

// GetRelationshipTypeByCodeTx loads a relationship type by its code using q.
// Returns ErrNotFound when the code is unknown.
func GetRelationshipTypeByCodeTx(ctx context.Context, q Querier, code string) (*RelationshipType, error) {
	rows, err := q.Query(ctx, getRelationshipTypeByCodeSQL, code)
	if err != nil {
		return nil, err
	}
	rt, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[RelationshipType])
	if err != nil {
		return nil, mapNotFound(err)
	}
	return rt, nil
}

// GetSubjectRefTx loads a subject_ref by id using q. Returns ErrNotFound when absent.
func GetSubjectRefTx(ctx context.Context, q Querier, id uuid.UUID) (*SubjectRef, error) {
	rows, err := q.Query(ctx, getSubjectRefSQL, id)
	if err != nil {
		return nil, err
	}
	ref, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[SubjectRef])
	if err != nil {
		return nil, mapNotFound(err)
	}
	return ref, nil
}

// GetRecordMetadataTx loads the governance record for a subject using q. Returns ErrNotFound when absent.
func GetRecordMetadataTx(ctx context.Context, q Querier, subjectID uuid.UUID) (*RecordMetadata, error) {
	rows, err := q.Query(ctx, getRecordMetadataSQL, subjectID)
	if err != nil {
		return nil, err
	}
	md, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[RecordMetadata])
	if err != nil {
		return nil, mapNotFound(err)
	}
	return md, nil
}

// LockRecordMetadataTx sets the immutable flag on a subject's governance record using q.
func LockRecordMetadataTx(ctx context.Context, q Querier, subjectID uuid.UUID, actorUserID string) (*RecordMetadata, error) {
	rows, err := q.Query(ctx, lockRecordMetadataSQL, subjectID, actorUserID)
	if err != nil {
		return nil, err
	}
	md, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[RecordMetadata])
	if err != nil {
		return nil, mapNotFound(err)
	}
	return md, nil
}

// SoftDeleteRecordMetadataTx logically deletes a subject's governance record using q.
func SoftDeleteRecordMetadataTx(ctx context.Context, q Querier, subjectID uuid.UUID, actorUserID string) (*RecordMetadata, error) {
	rows, err := q.Query(ctx, softDeleteRecordMetadataSQL, subjectID, actorUserID)
	if err != nil {
		return nil, err
	}
	md, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[RecordMetadata])
	if err != nil {
		return nil, mapNotFound(err)
	}
	return md, nil
}

// LinkSubjectsTx validates kind compatibility and inserts a typed relationship using q.
// It enforces (via the DB partial unique index) that no identical active edge exists.
func LinkSubjectsTx(ctx context.Context, q Querier, in LinkInput) (*SubjectRelationship, error) {
	source, err := GetSubjectRefTx(ctx, q, in.SourceSubjectID)
	if err != nil {
		return nil, err
	}
	target, err := GetSubjectRefTx(ctx, q, in.TargetSubjectID)
	if err != nil {
		return nil, err
	}
	rt, err := GetRelationshipTypeByCodeTx(ctx, q, in.RelationshipTypeCode)
	if err != nil {
		return nil, err
	}
	if source.Kind != rt.SourceKind || target.Kind != rt.TargetKind {
		return nil, ErrKindMismatch
	}
	rows, err := q.Query(ctx, insertSubjectRelationshipSQL,
		in.SourceSubjectID, in.TargetSubjectID, rt.ID, in.RoleDetail, in.ValidFrom, in.ActorUserID)
	if err != nil {
		return nil, mapConflict(err)
	}
	rel, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[SubjectRelationship])
	if err != nil {
		return nil, err
	}
	rel.Source = source
	rel.Target = target
	rel.RelationshipType = rt
	return rel, nil
}
