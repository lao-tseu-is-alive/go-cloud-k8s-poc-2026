package core

import (
	"time"

	"github.com/google/uuid"
)

// SubjectKind mirrors the subject_kind.code controlled list and the SubjectKind proto enum.
type SubjectKind string

const (
	// SubjectKindUnspecified is the zero value; never persisted.
	SubjectKindUnspecified SubjectKind = ""
	// SubjectKindCase identifies an administrative case file (affaire).
	SubjectKindCase SubjectKind = "CASE"
	// SubjectKindDocument identifies a GED document (pkg/document).
	SubjectKindDocument SubjectKind = "DOCUMENT"
	// SubjectKindThing identifies a physical object such as a parcel or building.
	SubjectKindThing SubjectKind = "THING"
	// SubjectKindActor identifies an external person or organization (pkg/actor);
	// never an authenticated operator.
	SubjectKindActor SubjectKind = "ACTOR"
	// SubjectKindUser identifies an internal system user as a graph subject.
	SubjectKindUser SubjectKind = "USER"
	// SubjectKindOrgUnit identifies an internal organizational unit.
	SubjectKindOrgUnit SubjectKind = "ORG_UNIT"
)

// Valid reports whether k is one of the known subject kinds.
func (k SubjectKind) Valid() bool {
	switch k {
	case SubjectKindCase, SubjectKindDocument, SubjectKindThing,
		SubjectKindActor, SubjectKindUser, SubjectKindOrgUnit:
		return true
	default:
		return false
	}
}

// SubjectRef is the canonical identity of any business subject.
//
// The `db` struct tags drive pgx named scanning (RowToStructByNameLax). Keep them
// in sync with the column projections in sql.go.
type SubjectRef struct {
	// ID is the server-generated subject identity, shared 1:1 by the domain
	// row (document, actor, ...) and its record_metadata.
	ID uuid.UUID `db:"id"`
	// Kind is immutable; (ID, Kind) is unique so domain tables can pin it.
	Kind SubjectKind `db:"kind"`
	// DisplayLabel is the non-blank human label, kept in sync with the domain
	// entity's own label (e.g. a document title) for graph projections.
	DisplayLabel string `db:"display_label"`
	// CanonicalURL is an optional stable link to the subject; empty when unset.
	CanonicalURL string `db:"canonical_url"`
	// BusinessRef is the optional human business reference (e.g. 2026-001245);
	// empty when none. It is not the identity: ID is.
	BusinessRef string `db:"business_ref"`
	// BusinessRefNamespace scopes BusinessRef (e.g. OPC); the pair is unique when
	// the namespace is set. Empty for a free reference or no reference.
	BusinessRefNamespace string `db:"business_ref_namespace"`
	// CreatedAt is the database insertion time.
	CreatedAt time.Time `db:"created_at"`
}

// RecordMetadata is the 1:1 governance record attached to every subject.
// Nullable timestamp columns use pointers so pgx can scan SQL NULLs.
//
// Operator identifiers (*By fields, OwnerUserID) always come from the
// authenticated caller via OperatorID, never from request payloads; an empty
// string means "not set" because those columns are NOT NULL DEFAULT ”.
type RecordMetadata struct {
	// SubjectID is the governed subject; also this row's primary key.
	SubjectID uuid.UUID `db:"subject_id"`
	// CreatedAt is the database insertion time.
	CreatedAt time.Time `db:"created_at"`
	// CreatedBy is the operator who created the subject.
	CreatedBy string `db:"created_by"`
	// UpdatedAt is the last mutation time; nil until the first update.
	UpdatedAt *time.Time `db:"updated_at"`
	// UpdatedBy is the operator of the last mutation; empty until then.
	UpdatedBy string `db:"updated_by"`
	// DeletedAt marks a logical (soft) delete; nil while the subject is live.
	// Rows are never physically deleted.
	DeletedAt *time.Time `db:"deleted_at"`
	// DeletedBy is the operator who soft-deleted the subject; empty while live.
	DeletedBy string `db:"deleted_by"`
	// OwnerUserID is the owning user; empty when unowned.
	OwnerUserID string `db:"owner_user_id"`
	// OwnerOrgID is the owning organizational unit; empty when unset.
	OwnerOrgID string `db:"owner_org_id"`
	// ConfidentialityLevel ranges from 0 (public) to 5 (most restricted),
	// enforced by a CHECK constraint. It is recorded but not yet enforced by a
	// permission engine.
	ConfidentialityLevel int32 `db:"confidentiality_level"`
	// Version starts at 1 and is incremented by every governance mutation.
	Version int32 `db:"version"`
	// IsLocked makes the subject immutable; mutations then fail with ErrLocked.
	IsLocked bool `db:"is_locked"`
	// LockedAt is the lock time; nil while unlocked.
	LockedAt *time.Time `db:"locked_at"`
	// LockedBy is the operator who locked the subject; empty while unlocked.
	LockedBy string `db:"locked_by"`
	// RetentionUntil is the retention deadline as an ISO date or a records
	// policy reference; empty when no retention rule applies.
	RetentionUntil string `db:"retention_until"`
	// SortFinal is the archival disposition: CONSERVER, ELIMINER, ARCHIVER or
	// VERSER_SAE; empty when undecided.
	SortFinal string `db:"sort_final"`
	// Metadata is secondary JSONB extension data; never holds critical fields.
	Metadata map[string]string `db:"metadata"`
}

// AuditEvent is an append-only probative audit record.
//
// Every mutation writes one in the same transaction as the change it records;
// rows are never updated or deleted.
type AuditEvent struct {
	// ID is the server-generated event identity.
	ID uuid.UUID `db:"id"`
	// SubjectID is the subject the event is about.
	SubjectID uuid.UUID `db:"subject_id"`
	// EventType is a non-blank upper-case code such as SUBJECT_CREATED or
	// DOCUMENT_FINALIZED.
	EventType string `db:"event_type"`
	// ActorUserID is the authenticated operator who caused the event (not a
	// domain ACTOR subject), derived server-side via OperatorID.
	ActorUserID string `db:"actor_user_id"`
	// OccurredAt is the database time of the event.
	OccurredAt time.Time `db:"occurred_at"`
	// BeforeState is a JSONB snapshot of the changed fields before the
	// mutation; nil for creations.
	BeforeState map[string]any `db:"before_state"`
	// AfterState is a JSONB snapshot of the changed fields after the mutation;
	// nil when not applicable.
	AfterState map[string]any `db:"after_state"`
	// Reason is the operator-supplied justification; empty when none was given.
	Reason string `db:"reason"`
	// CorrelationID groups events emitted by one business operation; nil when
	// the event stands alone.
	CorrelationID *uuid.UUID `db:"correlation_id"`
	// RequestID is the HTTP request identifier the event was written under;
	// empty outside a request.
	RequestID string `db:"request_id"`
	// Metadata is secondary JSONB context for the event.
	Metadata map[string]any `db:"metadata"`
}

// RelationshipType is an allowed typed relation between two subject kinds.
// Types are seeded reference data; a link is accepted only when the source and
// target subjects have exactly SourceKind and TargetKind.
type RelationshipType struct {
	// ID is the catalogue row identity.
	ID uuid.UUID `db:"id"`
	// Code is the unique, non-blank stable key used by APIs, e.g.
	// CASE_HAS_DOCUMENT.
	Code string `db:"code"`
	// Label is the human label read from source to target.
	Label string `db:"label"`
	// SourceKind is the required kind of the source subject.
	SourceKind SubjectKind `db:"source_kind"`
	// TargetKind is the required kind of the target subject.
	TargetKind SubjectKind `db:"target_kind"`
	// IsDirected reports whether the edge reads one way only.
	IsDirected bool `db:"is_directed"`
	// InverseLabel is the human label read from target to source; empty when
	// the type has none.
	InverseLabel string `db:"inverse_label"`
	// Description documents the business meaning of the type.
	Description string `db:"description"`
	// IsActive reports whether new links of this type may be created.
	IsActive bool `db:"is_active"`
}

// SubjectRelationship is an actual typed edge between two subjects. The related
// SubjectRef and RelationshipType are hydrated by the repository for read paths.
//
// At most one active (non-deleted) edge exists per (source, target, type);
// unlinking soft-deletes the edge so it can be recreated later.
type SubjectRelationship struct {
	// ID is the server-generated edge identity.
	ID uuid.UUID `db:"id"`
	// SourceSubjectID is the subject the edge starts from.
	SourceSubjectID uuid.UUID `db:"source_subject_id"`
	// TargetSubjectID is the subject the edge points to.
	TargetSubjectID uuid.UUID `db:"target_subject_id"`
	// RelationshipTypeID references the validated RelationshipType.
	RelationshipTypeID uuid.UUID `db:"relationship_type_id"`
	// RoleDetail qualifies the relation in free text; empty when not needed.
	RoleDetail string `db:"role_detail"`
	// ValidFrom is the optional business start of validity; nil when open.
	ValidFrom *time.Time `db:"valid_from"`
	// ValidTo is the business end of validity set by EndRelationship; nil while open.
	ValidTo *time.Time `db:"valid_to"`
	// CreatedAt is the database insertion time.
	CreatedAt time.Time `db:"created_at"`
	// CreatedBy is the operator who created the edge.
	CreatedBy string `db:"created_by"`
	// DeletedAt marks an unlinked (soft-deleted) edge; nil while active.
	DeletedAt *time.Time `db:"deleted_at"`

	// Source is the hydrated source subject on read paths; nil on writes.
	Source *SubjectRef `db:"-"`
	// Target is the hydrated target subject on read paths; nil on writes.
	Target *SubjectRef `db:"-"`
	// RelationshipType is the hydrated type on read paths; nil on writes.
	RelationshipType *RelationshipType `db:"-"`
}

// CreateSubjectInput holds the client-controlled fields for a new subject + its governance record.
type CreateSubjectInput struct {
	// Kind is the required subject kind; it must be Valid.
	Kind SubjectKind
	// DisplayLabel is the required non-blank human label.
	DisplayLabel string
	// CanonicalURL is an optional stable link to the subject.
	CanonicalURL string
	// OperatorID is the authenticated caller, set server-side from OperatorID;
	// it becomes created_by and the audit actor.
	OperatorID string
	// OwnerUserID is the initial owning user; empty leaves the subject unowned
	// (the RPC adapters default it to the operator before calling the service).
	OwnerUserID string
	// OwnerOrgID is the initial owning organizational unit.
	OwnerOrgID string
	// ConfidentialityLevel is the initial level, 0 to 5.
	ConfidentialityLevel int32
	// RetentionUntil is the initial retention deadline (see RecordMetadata).
	RetentionUntil string
	// SortFinal is the initial archival disposition (see RecordMetadata).
	SortFinal string
	// Metadata is initial secondary extension data.
	Metadata map[string]string
	// BusinessRef optionally assigns a business reference in the same
	// transaction; the zero value assigns none.
	BusinessRef BusinessRefRequest
}

// LinkInput holds the fields required to create a typed relationship.
type LinkInput struct {
	// SourceSubjectID is the existing source subject.
	SourceSubjectID uuid.UUID
	// TargetSubjectID is the existing target subject.
	TargetSubjectID uuid.UUID
	// RelationshipTypeCode selects an active RelationshipType whose kinds must
	// match the source and target kinds, otherwise ErrKindMismatch.
	RelationshipTypeCode string
	// RoleDetail optionally qualifies the relation.
	RoleDetail string
	// OperatorID is the authenticated caller, set server-side; it becomes
	// created_by and the audit actor.
	OperatorID string
	// ValidFrom is the optional business start of validity.
	ValidFrom *time.Time
}

// EndInput ends an open relationship in the business sense.
type EndInput struct {
	// RelationshipID is the open edge to end.
	RelationshipID uuid.UUID
	// ValidTo is the business end of validity; nil means the database time.
	// It may lie in the future but not before the edge's ValidFrom.
	ValidTo *time.Time
	// OperatorID is the authenticated caller, set server-side; it becomes the
	// audit actor.
	OperatorID string
	// Reason is the justification recorded on the audit event.
	Reason string
}

// RelationshipFilter controls relationship listing for one subject.
// Unlinked (soft-deleted) edges are excluded; ended edges are returned as history.
type RelationshipFilter struct {
	// SubjectID is the subject whose edges are listed.
	SubjectID uuid.UUID
	// Outgoing selects edges whose source is SubjectID when true, and edges
	// whose target is SubjectID when false.
	Outgoing bool
	// RelationshipTypeCode restricts the list to one type; empty means any.
	RelationshipTypeCode string
	// Limit is the page size, normalized to [1, MaxPageSize] with
	// DefaultPageSize when zero.
	Limit int
	// Offset is the zero-based number of rows to skip; negative becomes 0.
	Offset int
}

// RelationshipResult holds a page of relationships and the total count before pagination.
type RelationshipResult struct {
	// Relationships is the requested page, hydrated with subjects and type.
	Relationships []*SubjectRelationship
	// TotalSize is the number of matching edges across all pages.
	TotalSize int32
}

// AuditFilter controls audit-event listing.
// Events are returned newest first.
type AuditFilter struct {
	// SubjectID is the subject whose history is listed.
	SubjectID uuid.UUID
	// EventType restricts the list to one event code; empty means any.
	EventType string
	// From is the inclusive lower bound on OccurredAt; nil means unbounded.
	From *time.Time
	// To is the inclusive upper bound on OccurredAt; nil means unbounded.
	To *time.Time
	// Limit is the page size, normalized like RelationshipFilter.Limit.
	Limit int
	// Offset is the zero-based number of rows to skip; negative becomes 0.
	Offset int
}

// AuditResult holds a page of audit events and the total count before pagination.
type AuditResult struct {
	// Events is the requested page, newest first.
	Events []*AuditEvent
	// TotalSize is the number of matching events across all pages.
	TotalSize int32
}
