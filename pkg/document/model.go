package document

import (
	"time"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Status mirrors the document.status column and the DocumentStatus proto enum.
type Status int16

// Persisted values are 1 to 4 (CHECK constraint); 0 exists only in the API.
const (
	// StatusUnspecified is the proto zero value; never persisted.
	StatusUnspecified Status = 0
	// StatusDraft is the initial, still editable state.
	StatusDraft Status = 1
	// StatusFinal marks a document whose current version is final.
	StatusFinal Status = 2
	// StatusSuperseded marks a document replaced by a newer version.
	StatusSuperseded Status = 3
	// StatusArchived marks a document handed over to archiving.
	StatusArchived Status = 4
)

// DocumentType is a controlled classification of documents.
// Types are seeded reference data; a document's type is fixed at creation.
type DocumentType struct {
	// ID is the catalogue row identity.
	ID uuid.UUID `db:"id"`
	// Code is the unique, non-blank stable key used by APIs.
	Code string `db:"code"`
	// Label is the human label.
	Label string `db:"label"`
	// Description documents the business meaning of the type.
	Description string `db:"description"`
	// Category groups types, e.g. ENTREE, SORTIE, PLAN, DECISION or
	// JUSTIFICATIF; empty when uncategorized.
	Category string `db:"category"`
	// IsActive reports whether the type is offered for new documents.
	IsActive bool `db:"is_active"`
}

// ContentBlob is binary content identified by its SHA-256 (spec v2 §17). The
// digest is unique, so identical bytes are stored once and may back several
// versions; the digest identifies content, never a business document.
type ContentBlob struct {
	// ID is the blob identity, returned by the upload endpoint and passed to
	// CreateDocument / AddVersion as the trusted reference to the content.
	ID uuid.UUID `db:"id"`
	// SHA256 is the lower-case hex digest computed by the server while storing.
	SHA256 string `db:"sha256"`
	// StorageRef is the URI of the bytes (internal://...); empty when Goéland
	// knows the content only by digest (bytes held by an external system).
	StorageRef string `db:"storage_ref"`
	// MimeType is the media type of the bytes; empty when unknown.
	MimeType string `db:"mime_type"`
	// FileSizeBytes is the size of the bytes, never negative.
	FileSizeBytes int64 `db:"file_size_bytes"`
	// CreatedAt is the database insertion time.
	CreatedAt time.Time `db:"created_at"`
	// CreatedBy is the operator whose upload first stored the content.
	CreatedBy string `db:"created_by"`
	// VerifiedAt is reserved for probative re-verification of the stored bytes
	// (roadmap GLD-021); nil until then.
	VerifiedAt *time.Time `db:"verified_at"`
}

// Version is a dated, append-only state of a document (spec v2 §18). A final
// (validated) or record version is immutable, enforced by a database trigger;
// versions are never deleted.
type Version struct {
	// ID is the version identity.
	ID uuid.UUID `db:"id"`
	// DocumentID is the owning document.
	DocumentID uuid.UUID `db:"document_id"`
	// VersionNo numbers the versions of one document from 1, without gaps.
	VersionNo int32 `db:"version_no"`
	// ContentBlobID references the content; nil for a metadata-only document or
	// an external reference.
	ContentBlobID *uuid.UUID `db:"content_blob_id"`
	// PageCount is the number of pages; 0 when unknown.
	PageCount int32 `db:"page_count"`
	// IsFinal reports a validated, business-final version (then immutable).
	IsFinal bool `db:"is_final"`
	// IsRecord declares a probative record; a record is always final.
	IsRecord bool `db:"is_record"`
	// ValidatedAt is when the version became final; nil while mutable.
	ValidatedAt *time.Time `db:"validated_at"`
	// ValidatedBy is the operator who made the version final; empty while mutable.
	ValidatedBy string `db:"validated_by"`
	// Metadata is secondary JSONB data about this version.
	Metadata map[string]any `db:"metadata"`
	// CreatedAt is the database insertion time.
	CreatedAt time.Time `db:"created_at"`
	// CreatedBy is the operator who added the version.
	CreatedBy string `db:"created_by"`

	// Content is the hydrated blob on read paths; nil without content.
	Content *ContentBlob `db:"-"`
}

// Document is the logical business document (1:1 with a DOCUMENT subject_ref);
// its content lives in versions (spec v2 §15-16).
// Nullable columns use pointers so pgx can scan SQL NULLs.
//
// The table's search_vector column is GENERATED ALWAYS by Postgres and is
// deliberately absent here: the application never writes or scans it.
type Document struct {
	// ID is the DOCUMENT subject_ref ID; a composite foreign key pins the kind.
	ID uuid.UUID `db:"id"`
	// DocumentTypeID references the DocumentType chosen at creation.
	DocumentTypeID uuid.UUID `db:"document_type_id"`
	// Title is the non-blank title, at most MaxTitleLength code points; it is
	// mirrored into the subject's display label.
	Title string `db:"title"`
	// Description is free text, at most MaxDescriptionLength code points.
	Description string `db:"description"`
	// OfficialDate is the document's legal or business date (a DATE column,
	// time part zero); nil when unknown.
	OfficialDate *time.Time `db:"official_date"`
	// ExternalSystem names the system of record for external documents
	// (alfresco, minio, sharepoint, goeland-legacy); empty when internal.
	ExternalSystem string `db:"external_system"`
	// ExternalID is the document identifier inside ExternalSystem.
	ExternalID string `db:"external_id"`
	// ExternalURL is a link to the document in its external system; it is
	// also the subject's canonical URL.
	ExternalURL string `db:"external_url"`
	// Language is the language of the content (e.g. fr), unrelated to the UI
	// locale; empty when unknown.
	Language string `db:"language"`
	// Status is the lifecycle state; FINAL mirrors a final current version.
	Status Status `db:"status"`
	// Metadata is secondary JSONB extension data; never holds critical fields.
	Metadata map[string]any `db:"metadata"`
	// CurrentVersionID is the explicit current version, set in the same
	// transaction as every new version; nil only transiently during creation.
	CurrentVersionID *uuid.UUID `db:"current_version_id"`
	// CreatedAt is the database insertion time.
	CreatedAt time.Time `db:"created_at"`
	// CreatedBy is the operator who created the document.
	CreatedBy string `db:"created_by"`
	// UpdatedAt is maintained by a database trigger on every update.
	UpdatedAt time.Time `db:"updated_at"`

	// Subject is the hydrated identity on read paths; nil on writes.
	Subject *core.SubjectRef `db:"-"`
	// RecordMetadata is the hydrated governance record on read paths; nil on writes.
	RecordMetadata *core.RecordMetadata `db:"-"`
	// Type is the hydrated document type on read paths; nil on writes.
	Type *DocumentType `db:"-"`
	// CurrentVersion is the hydrated current version (with its content).
	CurrentVersion *Version `db:"-"`
}

// CreateInput holds the client-controlled fields for a new document.
//
// Service.Create persists the subject, governance record, document, version 1,
// optional links and audit event in one transaction. When ContentBlobID names
// content already used by a live document, that document is reused instead
// (spec v2 §20, decision in IMPLEMENTATION_STATUS §3g): no new document is
// created and the new context is expressed by the optional case link.
type CreateInput struct {
	// DocumentTypeCode is the required code of the document type.
	DocumentTypeCode string
	// Title is required; surrounding whitespace is trimmed.
	Title string
	// Description is optional free text.
	Description string
	// OfficialDate is the optional legal or business date.
	OfficialDate *time.Time
	// ExternalSystem names the external system of record, if any.
	ExternalSystem string
	// ExternalID identifies the document inside ExternalSystem.
	ExternalID string
	// ExternalURL links to the external document and becomes the canonical URL.
	ExternalURL string
	// ContentBlobID is the content registered by the upload endpoint; nil for a
	// metadata-only document or an external reference.
	ContentBlobID *uuid.UUID
	// IsFinal creates version 1 already final (and the document FINAL).
	IsFinal bool
	// IsRecord declares version 1 a record, which also makes it final.
	IsRecord bool
	// Language is the language of the content.
	Language string
	// PageCount is the number of pages of version 1.
	PageCount int32
	// Metadata is secondary extension data.
	Metadata map[string]any
	// OperatorID is the authenticated caller, set server-side; it becomes
	// created_by, the default owner and the audit actor.
	OperatorID string
	// Governance carries the requested owner, confidentiality and records
	// fields; the service overwrites its identity fields from the document.
	Governance core.CreateSubjectInput
	// LinkToCaseID, when set, also creates a CASE_HAS_DOCUMENT link from that
	// case (to the new or the reused document).
	LinkToCaseID *uuid.UUID
}

// CreateResult reports what Service.Create did.
type CreateResult struct {
	// Document is the created or reused document, hydrated.
	Document *Document
	// Event is DOCUMENT_CREATED, or DOCUMENT_REUSED on the existing document.
	Event *core.AuditEvent
	// Relationship is the CASE_HAS_DOCUMENT edge; nil without LinkToCaseID or
	// when the reused document was already linked to that case.
	Relationship *core.SubjectRelationship
	// Reused is true when an existing document with the same content was reused.
	Reused bool
}

// VersionInput holds the fields of a new document version.
type VersionInput struct {
	// ContentBlobID is the content of the version; nil for a metadata-only
	// version. It may repeat the content of an earlier version (v2 §19).
	ContentBlobID *uuid.UUID
	// IsFinal creates the version already final.
	IsFinal bool
	// IsRecord declares the version a record, which also makes it final.
	IsRecord bool
	// PageCount is the number of pages.
	PageCount int32
	// Metadata is secondary data about the version.
	Metadata map[string]any
	// OperatorID is the authenticated caller, set server-side.
	OperatorID string
	// Reason is the justification recorded on the audit event.
	Reason string
}

// IngestResult reports what Service.IngestContent stored.
type IngestResult struct {
	// Blob is the content registered for the uploaded bytes.
	Blob *ContentBlob
	// Reused is true when identical content was already known: the new bytes
	// were discarded and the existing blob returned (global deduplication).
	Reused bool
	// Filename is the original client filename, informational only.
	Filename string
}

// UpdateInput holds the mutable metadata of a document.
//
// Every field replaces the stored value (no partial merge); the update fails
// with core.ErrLocked or core.ErrDeleted when the record is not mutable.
type UpdateInput struct {
	// Title is required; surrounding whitespace is trimmed.
	Title string
	// Description replaces the description.
	Description string
	// OfficialDate replaces the official date; nil clears it.
	OfficialDate *time.Time
	// Language replaces the content language.
	Language string
	// Metadata replaces the whole extension map.
	Metadata map[string]any
	// OperatorID is the authenticated caller, set server-side.
	OperatorID string
	// Reason is the justification recorded on the audit event.
	Reason string
}

// SearchFilter controls document search. Results are newest first.
type SearchFilter struct {
	// Query is an accent-insensitive full-text query over title and
	// description; empty matches every document.
	Query string
	// DocumentTypeCode restricts results to one type; empty means any.
	DocumentTypeCode string
	// CaseID restricts results to documents of that case through an active
	// CASE_HAS_DOCUMENT link; nil means any.
	CaseID *uuid.UUID
	// ThingID restricts results to documents with an active outgoing link to
	// that thing; nil means any.
	ThingID *uuid.UUID
	// ConfidentialityMax is the inclusive upper bound on the confidentiality
	// level; 0 or less means no cap (5).
	ConfidentialityMax int32
	// OnlyRecords restricts results to documents whose current version is a record.
	OnlyRecords bool
	// OnlyFinal restricts results to documents whose current version is final.
	OnlyFinal bool
	// IncludeDeleted also returns soft-deleted documents.
	IncludeDeleted bool
	// Limit is the page size, normalized to [1, core.MaxPageSize].
	Limit int
	// Offset is the zero-based number of rows to skip; negative becomes 0.
	Offset int
}

// SearchResult holds a page of documents and the total count before pagination.
type SearchResult struct {
	// Documents is the requested page.
	Documents []*Document
	// TotalSize is the number of matching documents across all pages.
	TotalSize int32
}
