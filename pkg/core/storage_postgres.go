package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository implements Repository with pgx. Row scanning uses pgx named
// scanning driven by the `db` tags on the model structs.
type PostgresRepository struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

// NewPostgresRepository builds a PostgresRepository from a connection pool. A nil logger falls back to slog.Default.
func NewPostgresRepository(pool *pgxpool.Pool, log *slog.Logger) (*PostgresRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("%w: PostgreSQL pool is required", ErrInvalidInput)
	}
	if log == nil {
		log = slog.Default()
	}
	return &PostgresRepository{pool: pool, log: log}, nil
}

// Pool exposes the underlying connection pool so other domain repositories in the
// same bundle can share a single pool and compose transactions with core helpers.
func (r *PostgresRepository) Pool() *pgxpool.Pool { return r.pool }

// CreateSubject inserts a subject_ref + its record_metadata and writes a SUBJECT_CREATED audit event, all in one transaction.
func (r *PostgresRepository) CreateSubject(ctx context.Context, in CreateSubjectInput) (*SubjectRef, *RecordMetadata, *AuditEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("begin create subject: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	ref, err := InsertSubjectRefTx(ctx, tx, in.Kind, in.DisplayLabel, in.CanonicalURL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("insert subject_ref: %w", err)
	}
	md, err := InsertRecordMetadataTx(ctx, tx, in, ref.ID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("insert record_metadata: %w", err)
	}
	after := map[string]any{"kind": string(ref.Kind), "display_label": ref.DisplayLabel}
	if !in.BusinessRef.IsZero() {
		if ref, err = AssignBusinessRefTx(ctx, tx, ref.ID, in.BusinessRef); err != nil {
			return nil, nil, nil, err
		}
		after["business_ref"] = ref.BusinessRef
		after["business_ref_namespace"] = ref.BusinessRefNamespace
	}
	ev, err := InsertAuditEventTx(ctx, tx, AuditEvent{
		SubjectID:   ref.ID,
		EventType:   "SUBJECT_CREATED",
		ActorUserID: in.OperatorID,
		AfterState:  after,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("insert audit_event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, nil, fmt.Errorf("commit create subject: %w", err)
	}
	return ref, md, ev, nil
}

// GetSubject loads a subject_ref by id.
func (r *PostgresRepository) GetSubject(ctx context.Context, id uuid.UUID) (*SubjectRef, error) {
	return GetSubjectRefTx(ctx, r.pool, id)
}

// GetRecordMetadata loads the governance record for a subject.
func (r *PostgresRepository) GetRecordMetadata(ctx context.Context, subjectID uuid.UUID) (*RecordMetadata, error) {
	rows, err := r.pool.Query(ctx, getRecordMetadataSQL, pgx.NamedArgs{"subject_id": subjectID})
	if err != nil {
		return nil, fmt.Errorf("get record_metadata: %w", err)
	}
	md, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[RecordMetadata])
	if err != nil {
		return nil, mapNotFound(err)
	}
	return md, nil
}

// AssignBusinessRef gives an existing subject its business reference and writes
// a BUSINESS_REF_ASSIGNED audit event in the same transaction.
func (r *PostgresRepository) AssignBusinessRef(ctx context.Context, subjectID uuid.UUID, req BusinessRefRequest, operatorID, reason string) (*SubjectRef, *AuditEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin assign business_ref: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	ref, err := AssignBusinessRefTx(ctx, tx, subjectID, req)
	if err != nil {
		return nil, nil, err
	}
	ev, err := InsertAuditEventTx(ctx, tx, AuditEvent{
		SubjectID:   subjectID,
		EventType:   "BUSINESS_REF_ASSIGNED",
		ActorUserID: operatorID,
		Reason:      reason,
		AfterState: map[string]any{
			"business_ref":           ref.BusinessRef,
			"business_ref_namespace": ref.BusinessRefNamespace,
			"allocated":              req.Allocate,
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("insert audit_event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit assign business_ref: %w", err)
	}
	return ref, ev, nil
}

// LookupSubjects returns at most limit subjects whose business reference
// matches filter exactly, oldest first.
func (r *PostgresRepository) LookupSubjects(ctx context.Context, filter LookupFilter, limit int) ([]*SubjectRef, error) {
	rows, err := r.pool.Query(ctx, lookupSubjectsByBusinessRefSQL, pgx.NamedArgs{
		"business_ref":           filter.BusinessRef,
		"business_ref_namespace": filter.Namespace,
		"kind":                   string(filter.Kind),
		"limit":                  limit,
	})
	if err != nil {
		return nil, fmt.Errorf("lookup subjects: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[SubjectRef])
}

// LinkSubjects validates and creates a typed relationship and writes a RELATIONSHIP_LINKED audit event.
func (r *PostgresRepository) LinkSubjects(ctx context.Context, in LinkInput) (*SubjectRelationship, *AuditEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin link subjects: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rel, err := LinkSubjectsTx(ctx, tx, in)
	if err != nil {
		return nil, nil, err
	}
	ev, err := InsertAuditEventTx(ctx, tx, AuditEvent{
		SubjectID:   in.SourceSubjectID,
		EventType:   "RELATIONSHIP_LINKED",
		ActorUserID: in.OperatorID,
		AfterState: map[string]any{
			"relationship_id": rel.ID.String(),
			"type":            in.RelationshipTypeCode,
			"target":          in.TargetSubjectID.String(),
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("insert audit_event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit link subjects: %w", err)
	}
	return rel, ev, nil
}

// UnlinkSubjects soft-deletes a relationship and writes a RELATIONSHIP_UNLINKED audit event.
func (r *PostgresRepository) UnlinkSubjects(ctx context.Context, relationshipID uuid.UUID, operatorID, reason string) (*SubjectRelationship, *AuditEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin unlink subjects: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, softDeleteRelationshipSQL, pgx.NamedArgs{
		"id":          relationshipID,
		"operator_id": operatorID,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("soft delete relationship: %w", err)
	}
	rel, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[SubjectRelationship])
	if err != nil {
		return nil, nil, mapNotFound(err)
	}
	ev, err := InsertAuditEventTx(ctx, tx, AuditEvent{
		SubjectID:   rel.SourceSubjectID,
		EventType:   "RELATIONSHIP_UNLINKED",
		ActorUserID: operatorID,
		Reason:      reason,
		BeforeState: map[string]any{"relationship_id": rel.ID.String()},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("insert audit_event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit unlink subjects: %w", err)
	}
	return rel, ev, nil
}

// AppendAuditEvent writes a standalone audit event outside any domain transaction.
func (r *PostgresRepository) AppendAuditEvent(ctx context.Context, ev AuditEvent) (*AuditEvent, error) {
	return InsertAuditEventTx(ctx, r.pool, ev)
}

// relationshipListRow adds the window total to the base relationship columns for search scanning.
type relationshipListRow struct {
	SubjectRelationship
	// TotalSize is the COUNT(*) OVER () window total, repeated on every row.
	TotalSize int32 `db:"total_count"`
}

// ListRelationships returns a page of active relationships for a subject, hydrated with subject refs and types.
func (r *PostgresRepository) ListRelationships(ctx context.Context, filter RelationshipFilter) (RelationshipResult, error) {
	rows, err := r.pool.Query(ctx, listRelationshipsSQL, pgx.NamedArgs{
		"subject_id":             filter.SubjectID,
		"outgoing":               filter.Outgoing,
		"relationship_type_code": filter.RelationshipTypeCode,
		"limit":                  filter.Limit,
		"offset":                 filter.Offset,
	})
	if err != nil {
		return RelationshipResult{}, fmt.Errorf("list relationships: %w", err)
	}
	listRows, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[relationshipListRow])
	if err != nil {
		return RelationshipResult{}, fmt.Errorf("read relationships: %w", err)
	}
	result := RelationshipResult{Relationships: make([]*SubjectRelationship, len(listRows))}
	for i := range listRows {
		rel := listRows[i].SubjectRelationship
		result.Relationships[i] = &rel
		result.TotalSize = listRows[i].TotalSize
	}
	if err := r.hydrateRelationships(ctx, result.Relationships); err != nil {
		return RelationshipResult{}, err
	}
	return result, nil
}

// hydrateRelationships fills Source, Target and RelationshipType for each edge using two batch queries.
func (r *PostgresRepository) hydrateRelationships(ctx context.Context, rels []*SubjectRelationship) error {
	if len(rels) == 0 {
		return nil
	}
	subjectIDs := make([]uuid.UUID, 0, len(rels)*2)
	typeIDs := make([]uuid.UUID, 0, len(rels))
	for _, rel := range rels {
		subjectIDs = append(subjectIDs, rel.SourceSubjectID, rel.TargetSubjectID)
		typeIDs = append(typeIDs, rel.RelationshipTypeID)
	}

	subjectRows, err := r.pool.Query(ctx, getSubjectRefsByIDsSQL, pgx.NamedArgs{"ids": subjectIDs})
	if err != nil {
		return fmt.Errorf("hydrate subjects: %w", err)
	}
	subjects, err := pgx.CollectRows(subjectRows, pgx.RowToAddrOfStructByNameLax[SubjectRef])
	if err != nil {
		return fmt.Errorf("read hydrate subjects: %w", err)
	}
	subjectByID := make(map[uuid.UUID]*SubjectRef, len(subjects))
	for _, s := range subjects {
		subjectByID[s.ID] = s
	}

	typeRows, err := r.pool.Query(ctx, getRelationshipTypesByIDsSQL, pgx.NamedArgs{"ids": typeIDs})
	if err != nil {
		return fmt.Errorf("hydrate relationship types: %w", err)
	}
	types, err := pgx.CollectRows(typeRows, pgx.RowToAddrOfStructByNameLax[RelationshipType])
	if err != nil {
		return fmt.Errorf("read hydrate relationship types: %w", err)
	}
	typeByID := make(map[uuid.UUID]*RelationshipType, len(types))
	for _, t := range types {
		typeByID[t.ID] = t
	}

	for _, rel := range rels {
		rel.Source = subjectByID[rel.SourceSubjectID]
		rel.Target = subjectByID[rel.TargetSubjectID]
		rel.RelationshipType = typeByID[rel.RelationshipTypeID]
	}
	return nil
}

// ListRelationshipTypes returns the catalogue of relationship types, optionally filtered.
func (r *PostgresRepository) ListRelationshipTypes(ctx context.Context, onlyActive bool, sourceKind, targetKind SubjectKind) ([]*RelationshipType, error) {
	rows, err := r.pool.Query(ctx, listRelationshipTypesSQL, pgx.NamedArgs{
		"only_active": onlyActive,
		"source_kind": string(sourceKind),
		"target_kind": string(targetKind),
	})
	if err != nil {
		return nil, fmt.Errorf("list relationship types: %w", err)
	}
	types, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[RelationshipType])
	if err != nil {
		return nil, fmt.Errorf("read relationship types: %w", err)
	}
	return types, nil
}

// auditListRow adds the window total to the base audit columns for search scanning.
type auditListRow struct {
	AuditEvent
	// TotalSize is the COUNT(*) OVER () window total, repeated on every row.
	TotalSize int32 `db:"total_count"`
}

// ListAuditEvents returns a page of audit events for a subject, newest first.
func (r *PostgresRepository) ListAuditEvents(ctx context.Context, filter AuditFilter) (AuditResult, error) {
	rows, err := r.pool.Query(ctx, listAuditEventsSQL, pgx.NamedArgs{
		"subject_id": filter.SubjectID,
		"event_type": filter.EventType,
		"from":       filter.From,
		"to":         filter.To,
		"limit":      filter.Limit,
		"offset":     filter.Offset,
	})
	if err != nil {
		return AuditResult{}, fmt.Errorf("list audit events: %w", err)
	}
	listRows, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[auditListRow])
	if err != nil {
		return AuditResult{}, fmt.Errorf("read audit events: %w", err)
	}
	result := AuditResult{Events: make([]*AuditEvent, len(listRows))}
	for i := range listRows {
		ev := listRows[i].AuditEvent
		result.Events[i] = &ev
		result.TotalSize = listRows[i].TotalSize
	}
	return result, nil
}

// mapNotFound translates pgx.ErrNoRows to the domain ErrNotFound.
func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// mapConflict translates a unique-violation into ErrConflict and a foreign-key violation into ErrNotFound.
func mapConflict(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return fmt.Errorf("%w: an active relationship already exists", ErrConflict)
		case "23503": // foreign_key_violation
			return fmt.Errorf("%w: referenced subject or type does not exist", ErrNotFound)
		}
	}
	return err
}
