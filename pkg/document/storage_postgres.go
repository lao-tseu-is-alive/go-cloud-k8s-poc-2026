package document

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

// caseHasDocumentCode is the relationship type used to auto-link a new document to a case.
const caseHasDocumentCode = "CASE_HAS_DOCUMENT"

// PostgresRepository implements Repository with pgx, composing core primitives.
type PostgresRepository struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

// NewPostgresRepository builds a PostgresRepository from a connection pool.
func NewPostgresRepository(pool *pgxpool.Pool, log *slog.Logger) (*PostgresRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("%w: PostgreSQL pool is required", core.ErrInvalidInput)
	}
	if log == nil {
		log = slog.Default()
	}
	return &PostgresRepository{pool: pool, log: log}, nil
}

// Create inserts a document + its subject_ref + record_metadata + audit event (and
// an optional CASE_HAS_DOCUMENT link) in a single transaction.
func (r *PostgresRepository) Create(ctx context.Context, in CreateInput) (*Document, *core.AuditEvent, *core.SubjectRelationship, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("begin create document: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	ref, err := core.InsertSubjectRefTx(ctx, tx, core.SubjectKindDocument, in.Title, in.ExternalURL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("insert subject_ref: %w", err)
	}
	md, err := core.InsertRecordMetadataTx(ctx, tx, in.Governance, ref.ID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("insert record_metadata: %w", err)
	}
	docType, err := getDocumentTypeByCode(ctx, tx, in.DocumentTypeCode)
	if err != nil {
		return nil, nil, nil, err
	}

	rows, err := tx.Query(ctx, insertDocumentSQL,
		ref.ID, docType.ID, in.Title, in.Description, in.OfficialDate, in.StorageRef,
		in.ExternalSystem, in.ExternalID, in.ExternalURL, in.MimeType, in.FileSizeBytes, nullableString(in.SHA256),
		normalizeVersion(in.Version), in.PreviousVersionID, in.IsFinal, in.IsRecord, in.Language, in.PageCount,
		statusForInsert(in.IsFinal), documentMetadata(in.Metadata), in.ActorUserID)
	if err != nil {
		return nil, nil, nil, mapDBError(err)
	}
	doc, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Document])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("insert document: %w", err)
	}

	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   ref.ID,
		EventType:   "DOCUMENT_CREATED",
		ActorUserID: in.ActorUserID,
		AfterState:  map[string]any{"title": doc.Title, "document_type": docType.Code},
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("insert audit_event: %w", err)
	}

	var rel *core.SubjectRelationship
	if in.LinkToCaseID != nil && *in.LinkToCaseID != uuid.Nil {
		rel, err = core.LinkSubjectsTx(ctx, tx, core.LinkInput{
			SourceSubjectID:      *in.LinkToCaseID,
			TargetSubjectID:      ref.ID,
			RelationshipTypeCode: caseHasDocumentCode,
			ActorUserID:          in.ActorUserID,
		})
		if err != nil {
			return nil, nil, nil, fmt.Errorf("auto-link document to case: %w", err)
		}
		if _, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
			SubjectID:   ref.ID,
			EventType:   "RELATIONSHIP_LINKED",
			ActorUserID: in.ActorUserID,
			AfterState:  map[string]any{"type": caseHasDocumentCode, "case": in.LinkToCaseID.String()},
		}); err != nil {
			return nil, nil, nil, fmt.Errorf("insert link audit_event: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, nil, fmt.Errorf("commit create document: %w", err)
	}
	doc.Subject = ref
	doc.RecordMetadata = md
	doc.Type = docType
	return doc, ev, rel, nil
}

// Get loads a document with its subject, governance and type hydrated.
func (r *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (*Document, error) {
	rows, err := r.pool.Query(ctx, getDocumentSQL, id)
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	doc, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Document])
	if err != nil {
		return nil, mapDBError(err)
	}
	if err := r.hydrate(ctx, doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// UpdateMetadata updates mutable metadata after verifying the record is not locked.
func (r *PostgresRepository) UpdateMetadata(ctx context.Context, id uuid.UUID, in UpdateInput) (*Document, *core.AuditEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin update document: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	md, err := core.GetRecordMetadataTx(ctx, tx, id)
	if err != nil {
		return nil, nil, err
	}
	if md.IsLocked {
		return nil, nil, core.ErrLocked
	}
	rows, err := tx.Query(ctx, updateDocumentMetadataSQL,
		id, in.Title, in.Description, in.OfficialDate, in.Language, documentMetadata(in.Metadata))
	if err != nil {
		return nil, nil, fmt.Errorf("update document metadata: %w", err)
	}
	doc, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Document])
	if err != nil {
		return nil, nil, mapDBError(err)
	}
	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   id,
		EventType:   "DOCUMENT_METADATA_UPDATED",
		ActorUserID: in.ActorUserID,
		Reason:      in.Reason,
		AfterState:  map[string]any{"title": doc.Title},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("insert audit_event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit update document: %w", err)
	}
	if err := r.hydrate(ctx, doc); err != nil {
		return nil, nil, err
	}
	return doc, ev, nil
}

// Finalize marks a document final and optionally locks its governance record.
func (r *PostgresRepository) Finalize(ctx context.Context, id uuid.UUID, actorUserID, reason string, alsoLock bool) (*Document, *core.AuditEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin finalize document: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, finalizeDocumentSQL, id)
	if err != nil {
		return nil, nil, fmt.Errorf("finalize document: %w", err)
	}
	doc, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Document])
	if err != nil {
		return nil, nil, mapDBError(err)
	}
	if alsoLock {
		if _, err := core.LockRecordMetadataTx(ctx, tx, id, actorUserID); err != nil {
			return nil, nil, err
		}
	}
	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   id,
		EventType:   "DOCUMENT_FINALIZED",
		ActorUserID: actorUserID,
		Reason:      reason,
		AfterState:  map[string]any{"is_final": true, "locked": alsoLock},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("insert audit_event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit finalize document: %w", err)
	}
	if err := r.hydrate(ctx, doc); err != nil {
		return nil, nil, err
	}
	return doc, ev, nil
}

// Verify compares the stored hash with expectedSHA256 (when provided) and records
// the verification timestamp. For the POC no file bytes are read; trust is based
// on the stored, previously-registered hash.
func (r *PostgresRepository) Verify(ctx context.Context, id uuid.UUID, expectedSHA256 string) (*Document, bool, error) {
	doc, err := r.Get(ctx, id)
	if err != nil {
		return nil, false, err
	}
	stored := ""
	if doc.SHA256 != nil {
		stored = *doc.SHA256
	}
	verified := stored != "" && (expectedSHA256 == "" || equalFold(stored, expectedSHA256))
	if !verified {
		return doc, false, nil
	}
	rows, err := r.pool.Query(ctx, touchDocumentVerifiedAtSQL, id)
	if err != nil {
		return nil, false, fmt.Errorf("touch verified_at: %w", err)
	}
	updated, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Document])
	if err != nil {
		return nil, false, mapDBError(err)
	}
	if err := r.hydrate(ctx, updated); err != nil {
		return nil, false, err
	}
	return updated, true, nil
}

// Link creates a typed relationship from the document, delegating to core.
func (r *PostgresRepository) Link(ctx context.Context, in core.LinkInput) (*core.SubjectRelationship, *core.AuditEvent, error) {
	coreRepo, err := core.NewPostgresRepository(r.pool, r.log)
	if err != nil {
		return nil, nil, err
	}
	return coreRepo.LinkSubjects(ctx, in)
}

// SoftDelete logically deletes the document via its governance record and writes an audit event.
func (r *PostgresRepository) SoftDelete(ctx context.Context, id uuid.UUID, actorUserID, reason string) (*core.AuditEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin delete document: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := core.SoftDeleteRecordMetadataTx(ctx, tx, id, actorUserID); err != nil {
		return nil, err
	}
	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   id,
		EventType:   "DOCUMENT_DELETED",
		ActorUserID: actorUserID,
		Reason:      reason,
	})
	if err != nil {
		return nil, fmt.Errorf("insert audit_event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit delete document: %w", err)
	}
	return ev, nil
}

// documentListRow adds the window total to the document columns for search scanning.
type documentListRow struct {
	Document
	TotalSize int32 `db:"total_count"`
}

// Search runs full-text + filtered search and hydrates the results.
func (r *PostgresRepository) Search(ctx context.Context, filter SearchFilter) (SearchResult, error) {
	rows, err := r.pool.Query(ctx, searchDocumentsSQL,
		filter.Query, filter.DocumentTypeCode, filter.ConfidentialityMax,
		filter.OnlyRecords, filter.OnlyFinal, filter.IncludeDeleted,
		filter.CaseID, filter.ThingID, filter.Limit, filter.Offset)
	if err != nil {
		return SearchResult{}, fmt.Errorf("search documents: %w", err)
	}
	listRows, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[documentListRow])
	if err != nil {
		return SearchResult{}, fmt.Errorf("read documents: %w", err)
	}
	result := SearchResult{Documents: make([]*Document, len(listRows))}
	for i := range listRows {
		doc := listRows[i].Document
		result.Documents[i] = &doc
		result.TotalSize = listRows[i].TotalSize
	}
	for _, doc := range result.Documents {
		if err := r.hydrate(ctx, doc); err != nil {
			return SearchResult{}, err
		}
	}
	return result, nil
}

// ListTypes returns the document type catalogue.
func (r *PostgresRepository) ListTypes(ctx context.Context, onlyActive bool) ([]*DocumentType, error) {
	rows, err := r.pool.Query(ctx, listDocumentTypesSQL, onlyActive)
	if err != nil {
		return nil, fmt.Errorf("list document types: %w", err)
	}
	types, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[DocumentType])
	if err != nil {
		return nil, fmt.Errorf("read document types: %w", err)
	}
	return types, nil
}

// hydrate fills Subject, RecordMetadata and Type on a document.
func (r *PostgresRepository) hydrate(ctx context.Context, doc *Document) error {
	ref, err := core.GetSubjectRefTx(ctx, r.pool, doc.ID)
	if err != nil {
		return fmt.Errorf("hydrate subject: %w", err)
	}
	md, err := core.GetRecordMetadataTx(ctx, r.pool, doc.ID)
	if err != nil {
		return fmt.Errorf("hydrate metadata: %w", err)
	}
	docType, err := getDocumentTypeByID(ctx, r.pool, doc.DocumentTypeID)
	if err != nil {
		return fmt.Errorf("hydrate type: %w", err)
	}
	doc.Subject = ref
	doc.RecordMetadata = md
	doc.Type = docType
	return nil
}

// getDocumentTypeByCode loads a document type by code using q. Returns ErrNotFound when unknown.
func getDocumentTypeByCode(ctx context.Context, q core.Querier, code string) (*DocumentType, error) {
	rows, err := q.Query(ctx, getDocumentTypeByCodeSQL, code)
	if err != nil {
		return nil, err
	}
	dt, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[DocumentType])
	if err != nil {
		return nil, mapDBError(err)
	}
	return dt, nil
}

// getDocumentTypeByID loads a document type by id using q.
func getDocumentTypeByID(ctx context.Context, q core.Querier, id uuid.UUID) (*DocumentType, error) {
	rows, err := q.Query(ctx, getDocumentTypesByIDsSQL, []uuid.UUID{id})
	if err != nil {
		return nil, err
	}
	dt, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[DocumentType])
	if err != nil {
		return nil, mapDBError(err)
	}
	return dt, nil
}

// --- small helpers -----------------------------------------------------------

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func documentMetadata(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

func normalizeVersion(v int32) int32 {
	if v <= 0 {
		return 1
	}
	return v
}

func statusForInsert(isFinal bool) int16 {
	if isFinal {
		return int16(StatusFinal)
	}
	return int16(StatusDraft)
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return toLowerHex(a) == toLowerHex(b)
}

func toLowerHex(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'F' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

// mapDBError translates pgx.ErrNoRows to core.ErrNotFound and preserves conflict/FK mapping.
func mapDBError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return core.ErrNotFound
	}
	return err
}
