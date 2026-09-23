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

// caseHasDocumentCode is the relationship type used by document creation for
// atomic auto-linking to a case.
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

// Create inserts a document with its subject_ref, record_metadata, version 1 and
// audit event (and an optional CASE_HAS_DOCUMENT link) in a single transaction,
// or reuses the live document that already holds the same content.
func (r *PostgresRepository) Create(ctx context.Context, in CreateInput) (CreateResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return CreateResult{}, fmt.Errorf("begin create document: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var blob *ContentBlob
	if in.ContentBlobID != nil {
		if blob, err = getBlob(ctx, tx, *in.ContentBlobID); err != nil {
			return CreateResult{}, fmt.Errorf("content blob: %w", err)
		}
		existing, err := findReusableDocument(ctx, tx, blob.ID)
		if err != nil {
			return CreateResult{}, err
		}
		if existing != nil {
			res, err := r.reuse(ctx, tx, existing, blob, in)
			if err != nil {
				return CreateResult{}, err
			}
			if err := tx.Commit(ctx); err != nil {
				return CreateResult{}, fmt.Errorf("commit reuse document: %w", err)
			}
			return res, r.hydrate(ctx, res.Document)
		}
	}

	ref, err := core.InsertSubjectRefTx(ctx, tx, core.SubjectKindDocument, in.Title, in.ExternalURL)
	if err != nil {
		return CreateResult{}, fmt.Errorf("insert subject_ref: %w", err)
	}
	if _, err := core.InsertRecordMetadataTx(ctx, tx, in.Governance, ref.ID); err != nil {
		return CreateResult{}, fmt.Errorf("insert record_metadata: %w", err)
	}
	docType, err := getDocumentTypeByCode(ctx, tx, in.DocumentTypeCode)
	if err != nil {
		return CreateResult{}, err
	}
	if !docType.IsActive {
		return CreateResult{}, fmt.Errorf("%w: document type %q is inactive", core.ErrInvalidInput, in.DocumentTypeCode)
	}
	rows, err := tx.Query(ctx, insertDocumentSQL, pgx.NamedArgs{
		"id":               ref.ID,
		"document_type_id": docType.ID,
		"title":            in.Title,
		"description":      in.Description,
		"official_date":    in.OfficialDate,
		"external_system":  in.ExternalSystem,
		"external_id":      in.ExternalID,
		"external_url":     in.ExternalURL,
		"language":         in.Language,
		"status":           int16(StatusDraft),
		"metadata":         jsonMap(in.Metadata),
		"created_by":       in.OperatorID,
	})
	if err != nil {
		return CreateResult{}, mapDBError(err)
	}
	if _, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Document]); err != nil {
		return CreateResult{}, fmt.Errorf("insert document: %w", err)
	}
	doc, version, err := addVersion(ctx, tx, ref.ID, VersionInput{
		ContentBlobID: in.ContentBlobID,
		IsFinal:       in.IsFinal,
		IsRecord:      in.IsRecord,
		PageCount:     in.PageCount,
		OperatorID:    in.OperatorID,
	})
	if err != nil {
		return CreateResult{}, err
	}

	after := map[string]any{"title": doc.Title, "document_type": docType.Code, "version_no": version.VersionNo}
	if blob != nil {
		after["content_blob_id"] = blob.ID.String()
		after["sha256"] = blob.SHA256
	}
	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   ref.ID,
		EventType:   "DOCUMENT_CREATED",
		ActorUserID: in.OperatorID,
		AfterState:  after,
	})
	if err != nil {
		return CreateResult{}, fmt.Errorf("insert audit_event: %w", err)
	}
	rel, err := linkToCase(ctx, tx, ref.ID, in)
	if err != nil {
		return CreateResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CreateResult{}, fmt.Errorf("commit create document: %w", err)
	}
	res := CreateResult{Document: doc, Event: ev, Relationship: rel}
	return res, r.hydrate(ctx, doc)
}

// reuse records the reuse of an existing document for identical content: no new
// document, a DOCUMENT_REUSED audit event on the existing one, and the optional
// case link (an already active link is kept as is).
func (r *PostgresRepository) reuse(ctx context.Context, tx pgx.Tx, existing *Document, blob *ContentBlob, in CreateInput) (CreateResult, error) {
	after := map[string]any{
		"content_blob_id": blob.ID.String(),
		"sha256":          blob.SHA256,
		"requested_title": in.Title,
	}
	if in.LinkToCaseID != nil {
		after["case"] = in.LinkToCaseID.String()
	}
	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   existing.ID,
		EventType:   "DOCUMENT_REUSED",
		ActorUserID: in.OperatorID,
		AfterState:  after,
	})
	if err != nil {
		return CreateResult{}, fmt.Errorf("insert audit_event: %w", err)
	}
	// A failed statement aborts a PostgreSQL transaction, so the link attempt runs
	// in a savepoint: an already active link to this case is kept as is.
	savepoint, err := tx.Begin(ctx)
	if err != nil {
		return CreateResult{}, fmt.Errorf("begin link savepoint: %w", err)
	}
	rel, err := linkToCase(ctx, savepoint, existing.ID, in)
	switch {
	case errors.Is(err, core.ErrConflict):
		rel = nil
		if err := savepoint.Rollback(ctx); err != nil {
			return CreateResult{}, fmt.Errorf("rollback link savepoint: %w", err)
		}
	case err != nil:
		return CreateResult{}, err
	default:
		if err := savepoint.Commit(ctx); err != nil {
			return CreateResult{}, fmt.Errorf("release link savepoint: %w", err)
		}
	}
	r.log.Info("reused document for identical content", "document_id", existing.ID, "content_blob_id", blob.ID)
	return CreateResult{Document: existing, Event: ev, Relationship: rel, Reused: true}, nil
}

// linkToCase creates the optional CASE_HAS_DOCUMENT edge and its audit event.
func linkToCase(ctx context.Context, tx pgx.Tx, documentID uuid.UUID, in CreateInput) (*core.SubjectRelationship, error) {
	if in.LinkToCaseID == nil || *in.LinkToCaseID == uuid.Nil {
		return nil, nil
	}
	rel, err := core.LinkSubjectsTx(ctx, tx, core.LinkInput{
		SourceSubjectID:      *in.LinkToCaseID,
		TargetSubjectID:      documentID,
		RelationshipTypeCode: caseHasDocumentCode,
		OperatorID:           in.OperatorID,
	})
	if err != nil {
		return nil, fmt.Errorf("auto-link document to case: %w", err)
	}
	if _, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   documentID,
		EventType:   "RELATIONSHIP_LINKED",
		ActorUserID: in.OperatorID,
		AfterState:  map[string]any{"type": caseHasDocumentCode, "case": in.LinkToCaseID.String()},
	}); err != nil {
		return nil, fmt.Errorf("insert link audit_event: %w", err)
	}
	return rel, nil
}

// addVersion appends a version to documentID and makes it current, aligning the
// document status. The caller holds the transaction and, for existing
// documents, the governance row lock.
func addVersion(ctx context.Context, tx pgx.Tx, documentID uuid.UUID, in VersionInput) (*Document, *Version, error) {
	final := in.IsFinal || in.IsRecord
	rows, err := tx.Query(ctx, insertVersionSQL, pgx.NamedArgs{
		"document_id":     documentID,
		"content_blob_id": in.ContentBlobID,
		"page_count":      in.PageCount,
		"is_final":        final,
		"is_record":       in.IsRecord,
		"metadata":        jsonMap(in.Metadata),
		"created_by":      in.OperatorID,
	})
	if err != nil {
		return nil, nil, mapDBError(err)
	}
	version, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Version])
	if err != nil {
		return nil, nil, mapDBError(err)
	}
	status := StatusDraft
	if final {
		status = StatusFinal
	}
	rows, err = tx.Query(ctx, setCurrentVersionSQL, pgx.NamedArgs{"id": documentID, "version_id": version.ID, "status": int16(status)})
	if err != nil {
		return nil, nil, fmt.Errorf("set current version: %w", err)
	}
	doc, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Document])
	if err != nil {
		return nil, nil, mapDBError(err)
	}
	return doc, version, nil
}

// AddVersion appends a version to a mutable document, makes it current and
// writes a DOCUMENT_VERSION_ADDED audit event in one transaction.
func (r *PostgresRepository) AddVersion(ctx context.Context, documentID uuid.UUID, in VersionInput) (*Document, *Version, *core.AuditEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("begin add version: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Reject a locked or soft-deleted document; the row lock also serializes
	// concurrent additions so version numbers stay gap-free.
	if _, err := core.EnsureMutableTx(ctx, tx, documentID, false); err != nil {
		return nil, nil, nil, err
	}
	after := map[string]any{}
	if in.ContentBlobID != nil {
		blob, err := getBlob(ctx, tx, *in.ContentBlobID)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("content blob: %w", err)
		}
		after["content_blob_id"] = blob.ID.String()
		after["sha256"] = blob.SHA256
	}
	doc, version, err := addVersion(ctx, tx, documentID, in)
	if err != nil {
		return nil, nil, nil, err
	}
	after["version_no"] = version.VersionNo
	after["is_final"] = version.IsFinal
	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   documentID,
		EventType:   "DOCUMENT_VERSION_ADDED",
		ActorUserID: in.OperatorID,
		Reason:      in.Reason,
		AfterState:  after,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("insert audit_event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, nil, fmt.Errorf("commit add version: %w", err)
	}
	if err := r.hydrate(ctx, doc); err != nil {
		return nil, nil, nil, err
	}
	return doc, doc.CurrentVersion, ev, nil
}

// ListVersions returns every version of a document, newest first, with content.
func (r *PostgresRepository) ListVersions(ctx context.Context, documentID uuid.UUID) ([]*Version, error) {
	if _, err := core.GetSubjectRefTx(ctx, r.pool, documentID); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, listVersionsSQL, pgx.NamedArgs{"document_id": documentID})
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	versions, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[Version])
	if err != nil {
		return nil, fmt.Errorf("read versions: %w", err)
	}
	for _, v := range versions {
		if err := hydrateVersion(ctx, r.pool, v); err != nil {
			return nil, err
		}
	}
	return versions, nil
}

// RegisterBlob stores the metadata of freshly written content, or returns the
// blob that already holds the digest (reused = true) when identical content is
// known, including when a concurrent upload won the race.
func (r *PostgresRepository) RegisterBlob(ctx context.Context, blob ContentBlob) (*ContentBlob, bool, error) {
	rows, err := r.pool.Query(ctx, insertBlobSQL, pgx.NamedArgs{
		"sha256":          blob.SHA256,
		"storage_ref":     blob.StorageRef,
		"mime_type":       blob.MimeType,
		"file_size_bytes": blob.FileSizeBytes,
		"created_by":      blob.CreatedBy,
	})
	if err != nil {
		return nil, false, mapDBError(err)
	}
	inserted, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[ContentBlob])
	if err == nil {
		return inserted, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, mapDBError(err)
	}
	existing, err := r.FindBlobBySHA256(ctx, blob.SHA256)
	if err != nil {
		return nil, false, err
	}
	if existing.FileSizeBytes != blob.FileSizeBytes {
		// Same SHA-256, different size: storage corruption or a hash collision.
		return nil, false, fmt.Errorf("content blob %s: size %d does not match registered size %d", existing.ID, blob.FileSizeBytes, existing.FileSizeBytes)
	}
	return existing, true, nil
}

// FindBlobBySHA256 returns the blob holding a digest, or core.ErrNotFound.
func (r *PostgresRepository) FindBlobBySHA256(ctx context.Context, sha256 string) (*ContentBlob, error) {
	rows, err := r.pool.Query(ctx, getBlobBySHA256SQL, pgx.NamedArgs{"sha256": sha256})
	if err != nil {
		return nil, fmt.Errorf("find content blob: %w", err)
	}
	blob, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[ContentBlob])
	if err != nil {
		return nil, mapDBError(err)
	}
	return blob, nil
}

// Get loads a document with its subject, governance, type and current version hydrated.
func (r *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (*Document, error) {
	rows, err := r.pool.Query(ctx, getDocumentSQL, pgx.NamedArgs{"id": id})
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

	// Reject the mutation atomically if the record is locked or soft-deleted.
	if _, err := core.EnsureMutableTx(ctx, tx, id, false); err != nil {
		return nil, nil, err
	}
	rows, err := tx.Query(ctx, updateDocumentMetadataSQL, pgx.NamedArgs{
		"id":            id,
		"title":         in.Title,
		"description":   in.Description,
		"official_date": in.OfficialDate,
		"language":      in.Language,
		"metadata":      jsonMap(in.Metadata),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("update document metadata: %w", err)
	}
	doc, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Document])
	if err != nil {
		return nil, nil, mapDBError(err)
	}
	// Keep the canonical subject label in sync with the new title (QW5).
	if err := core.UpdateSubjectLabelTx(ctx, tx, id, doc.Title); err != nil {
		return nil, nil, fmt.Errorf("sync subject label: %w", err)
	}
	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   id,
		EventType:   "DOCUMENT_METADATA_UPDATED",
		ActorUserID: in.OperatorID,
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

// Finalize makes the current version final (once; repeating is idempotent), sets
// the document FINAL and optionally locks its governance record.
func (r *PostgresRepository) Finalize(ctx context.Context, id uuid.UUID, operatorID, reason string, alsoLock bool) (*Document, *core.AuditEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin finalize document: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Reject finalizing a locked (already immutable) or soft-deleted document.
	if _, err := core.EnsureMutableTx(ctx, tx, id, false); err != nil {
		return nil, nil, err
	}
	var versionNo int32
	rows, err := tx.Query(ctx, finalizeCurrentVersionSQL, pgx.NamedArgs{"document_id": id, "operator_id": operatorID})
	if err != nil {
		return nil, nil, fmt.Errorf("finalize current version: %w", err)
	}
	if v, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Version]); err == nil {
		versionNo = v.VersionNo
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, mapDBError(err)
	}
	rows, err = tx.Query(ctx, setDocumentStatusSQL, pgx.NamedArgs{"id": id, "status": int16(StatusFinal)})
	if err != nil {
		return nil, nil, fmt.Errorf("finalize document: %w", err)
	}
	doc, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Document])
	if err != nil {
		return nil, nil, mapDBError(err)
	}
	if alsoLock {
		if _, err := core.LockRecordMetadataTx(ctx, tx, id, operatorID); err != nil {
			return nil, nil, err
		}
	}
	after := map[string]any{"is_final": true, "locked": alsoLock}
	if versionNo > 0 {
		after["version_no"] = versionNo
	}
	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   id,
		EventType:   "DOCUMENT_FINALIZED",
		ActorUserID: operatorID,
		Reason:      reason,
		AfterState:  after,
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

// Verify performs an HONEST, NON-MUTATING stored-hash comparison against the
// content of the current version.
//
// It does NOT read any bytes from storage and writes nothing to the database,
// so it is deliberately non-probative: it only answers "does the caller's expected
// hash match the hash registered for this document's current content?". "verified"
// therefore requires BOTH a non-empty expected hash and a matching stored hash — a
// blank expected hash is never treated as verified (that would be false assurance).
//
// Real probative verification (streaming the stored bytes, recomputing SHA-256,
// writing an audited verification event under a write scope) is roadmap GLD-021.
func (r *PostgresRepository) Verify(ctx context.Context, id uuid.UUID, expectedSHA256 string) (*Document, bool, error) {
	doc, err := r.Get(ctx, id)
	if err != nil {
		return nil, false, err
	}
	stored := ""
	if doc.CurrentVersion != nil && doc.CurrentVersion.Content != nil {
		stored = doc.CurrentVersion.Content.SHA256
	}
	return doc, hashMatches(stored, expectedSHA256), nil
}

// hashMatches reports whether a caller's expected hash matches the stored hash.
// BOTH must be non-empty — a blank expected hash is never a match, so the check
// cannot produce false assurance.
func hashMatches(stored, expected string) bool {
	return expected != "" && stored != "" && equalFold(stored, expected)
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
func (r *PostgresRepository) SoftDelete(ctx context.Context, id uuid.UUID, operatorID, reason string) (*core.AuditEvent, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin delete document: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Reject deleting an already soft-deleted document (locked is allowed to be retired).
	if _, err := core.EnsureMutableTx(ctx, tx, id, true); err != nil {
		return nil, err
	}
	if _, err := core.SoftDeleteRecordMetadataTx(ctx, tx, id, operatorID); err != nil {
		return nil, err
	}
	ev, err := core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
		SubjectID:   id,
		EventType:   "DOCUMENT_DELETED",
		ActorUserID: operatorID,
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
	// TotalSize is the COUNT(*) OVER () window total, repeated on every row.
	TotalSize int32 `db:"total_count"`
}

// Search runs full-text + filtered search and hydrates the results.
func (r *PostgresRepository) Search(ctx context.Context, filter SearchFilter) (SearchResult, error) {
	rows, err := r.pool.Query(ctx, searchDocumentsSQL, pgx.NamedArgs{
		"query":               filter.Query,
		"document_type_code":  filter.DocumentTypeCode,
		"confidentiality_max": filter.ConfidentialityMax,
		"only_records":        filter.OnlyRecords,
		"only_final":          filter.OnlyFinal,
		"include_deleted":     filter.IncludeDeleted,
		"case_id":             filter.CaseID,
		"thing_id":            filter.ThingID,
		"limit":               filter.Limit,
		"offset":              filter.Offset,
	})
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
	rows, err := r.pool.Query(ctx, listDocumentTypesSQL, pgx.NamedArgs{"only_active": onlyActive})
	if err != nil {
		return nil, fmt.Errorf("list document types: %w", err)
	}
	types, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[DocumentType])
	if err != nil {
		return nil, fmt.Errorf("read document types: %w", err)
	}
	return types, nil
}

// hydrate fills Subject, RecordMetadata, Type and CurrentVersion on a document.
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
	if doc.CurrentVersionID != nil {
		version, err := getVersion(ctx, r.pool, *doc.CurrentVersionID)
		if err != nil {
			return fmt.Errorf("hydrate current version: %w", err)
		}
		doc.CurrentVersion = version
	}
	return nil
}

// getVersion loads a version with its content using q.
func getVersion(ctx context.Context, q core.Querier, id uuid.UUID) (*Version, error) {
	rows, err := q.Query(ctx, getVersionSQL, pgx.NamedArgs{"id": id})
	if err != nil {
		return nil, err
	}
	v, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Version])
	if err != nil {
		return nil, mapDBError(err)
	}
	return v, hydrateVersion(ctx, q, v)
}

// hydrateVersion fills the content blob of a version, if any.
func hydrateVersion(ctx context.Context, q core.Querier, v *Version) error {
	if v.ContentBlobID == nil {
		return nil
	}
	blob, err := getBlob(ctx, q, *v.ContentBlobID)
	if err != nil {
		return fmt.Errorf("hydrate content: %w", err)
	}
	v.Content = blob
	return nil
}

// getBlob loads a content blob by id using q; core.ErrNotFound when unknown.
func getBlob(ctx context.Context, q core.Querier, id uuid.UUID) (*ContentBlob, error) {
	rows, err := q.Query(ctx, getBlobSQL, pgx.NamedArgs{"id": id})
	if err != nil {
		return nil, err
	}
	blob, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[ContentBlob])
	if err != nil {
		return nil, mapDBError(err)
	}
	return blob, nil
}

// findReusableDocument returns the live document already holding the blob, or nil.
func findReusableDocument(ctx context.Context, q core.Querier, blobID uuid.UUID) (*Document, error) {
	rows, err := q.Query(ctx, findReusableDocumentSQL, pgx.NamedArgs{"content_blob_id": blobID})
	if err != nil {
		return nil, fmt.Errorf("find reusable document: %w", err)
	}
	doc, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Document])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return doc, nil
}

// getDocumentTypeByCode loads a document type by code using q. Returns ErrNotFound when unknown.
func getDocumentTypeByCode(ctx context.Context, q core.Querier, code string) (*DocumentType, error) {
	rows, err := q.Query(ctx, getDocumentTypeByCodeSQL, pgx.NamedArgs{"code": code})
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
	rows, err := q.Query(ctx, getDocumentTypesByIDsSQL, pgx.NamedArgs{"ids": []uuid.UUID{id}})
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

func jsonMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
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
