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
	// StatusFinal marks a finalized document (IsFinal is true).
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

// Document is the document-specific projection (1:1 with a DOCUMENT subject_ref).
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
	// StorageRef is the URI of the bytes (internal://, minio://, alfresco://);
	// empty for metadata-only documents.
	StorageRef string `db:"storage_ref"`
	// ExternalSystem names the system of record for external documents
	// (alfresco, minio, sharepoint, goeland-legacy); empty when internal.
	ExternalSystem string `db:"external_system"`
	// ExternalID is the document identifier inside ExternalSystem.
	ExternalID string `db:"external_id"`
	// ExternalURL is a link to the document in its external system; it is
	// also the subject's canonical URL.
	ExternalURL string `db:"external_url"`
	// MimeType is the media type of the bytes; empty when unknown.
	MimeType string `db:"mime_type"`
	// FileSizeBytes is the size of the bytes, never negative; 0 when unknown.
	FileSizeBytes int64 `db:"file_size_bytes"`
	// SHA256 is the registered hex digest of the bytes; nil when unknown.
	// Non-nil digests are unique across documents (deduplication key).
	SHA256 *string `db:"sha256"`
	// SHA256VerifiedAt is reserved for probative re-verification of the stored
	// bytes; the current Verify only compares hashes and never sets it.
	SHA256VerifiedAt *time.Time `db:"sha256_verified_at"`
	// Version is the document version, at least 1.
	Version int32 `db:"version"`
	// PreviousVersionID is the document this one supersedes; nil for a first
	// version. It is also recorded as a DOCUMENT_PREVIOUS_VERSION link.
	PreviousVersionID *uuid.UUID `db:"previous_version_id"`
	// IsFinal reports whether the document was finalized (Status FINAL).
	IsFinal bool `db:"is_final"`
	// IsRecord flags a probative record subject to records management.
	IsRecord bool `db:"is_record"`
	// Language is the language of the content (e.g. fr), unrelated to the UI
	// locale; empty when unknown.
	Language string `db:"language"`
	// PageCount is the number of pages; 0 when unknown.
	PageCount int32 `db:"page_count"`
	// Status is the lifecycle state.
	Status Status `db:"status"`
	// Metadata is secondary JSONB extension data; never holds critical fields.
	Metadata map[string]any `db:"metadata"`
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
}

// CreateInput holds the client-controlled fields for a new document.
//
// Service.Create persists the subject, governance record, document, optional
// links and audit event in one transaction. Fields mirror Document unless noted.
type CreateInput struct {
	// DocumentTypeCode is the required code of the document type.
	DocumentTypeCode string
	// Title is required; surrounding whitespace is trimmed.
	Title string
	// Description is optional free text.
	Description string
	// OfficialDate is the optional legal or business date.
	OfficialDate *time.Time
	// StorageRef is the bytes URI, e.g. the internal:// ref returned by upload.
	StorageRef string
	// ExternalSystem names the external system of record, if any.
	ExternalSystem string
	// ExternalID identifies the document inside ExternalSystem.
	ExternalID string
	// ExternalURL links to the external document and becomes the canonical URL.
	ExternalURL string
	// MimeType is the media type of the bytes.
	MimeType string
	// FileSizeBytes is the size of the bytes.
	FileSizeBytes int64
	// SHA256 is the hex digest; empty stores NULL.
	SHA256 string
	// Version is the document version; 0 or less becomes 1.
	Version int32
	// PreviousVersionID, when set, also creates a DOCUMENT_PREVIOUS_VERSION link.
	PreviousVersionID *uuid.UUID
	// IsFinal creates the document directly in Status FINAL instead of DRAFT.
	IsFinal bool
	// IsRecord flags the document as a probative record.
	IsRecord bool
	// Language is the language of the content.
	Language string
	// PageCount is the number of pages.
	PageCount int32
	// Metadata is secondary extension data.
	Metadata map[string]any
	// OperatorID is the authenticated caller, set server-side; it becomes
	// created_by, the default owner and the audit actor.
	OperatorID string
	// Governance carries the requested owner, confidentiality and records
	// fields; the service overwrites its identity fields from the document.
	Governance core.CreateSubjectInput
	// LinkToCaseID, when set, also creates a CASE_HAS_DOCUMENT link from that case.
	LinkToCaseID *uuid.UUID
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
	// OnlyRecords restricts results to probative records.
	OnlyRecords bool
	// OnlyFinal restricts results to finalized documents.
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
