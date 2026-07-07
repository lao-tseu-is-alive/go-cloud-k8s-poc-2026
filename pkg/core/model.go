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
	SubjectKindCase        SubjectKind = "CASE"
	SubjectKindDocument    SubjectKind = "DOCUMENT"
	SubjectKindThing       SubjectKind = "THING"
	SubjectKindActor       SubjectKind = "ACTOR"
	SubjectKindUser        SubjectKind = "USER"
	SubjectKindOrgUnit     SubjectKind = "ORG_UNIT"
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
	ID           uuid.UUID   `db:"id"`
	Kind         SubjectKind `db:"kind"`
	DisplayLabel string      `db:"display_label"`
	CanonicalURL string      `db:"canonical_url"`
	CreatedAt    time.Time   `db:"created_at"`
}

// RecordMetadata is the 1:1 governance record attached to every subject.
// Nullable timestamp columns use pointers so pgx can scan SQL NULLs.
type RecordMetadata struct {
	SubjectID            uuid.UUID         `db:"subject_id"`
	CreatedAt            time.Time         `db:"created_at"`
	CreatedBy            string            `db:"created_by"`
	UpdatedAt            *time.Time        `db:"updated_at"`
	UpdatedBy            string            `db:"updated_by"`
	DeletedAt            *time.Time        `db:"deleted_at"`
	DeletedBy            string            `db:"deleted_by"`
	OwnerUserID          string            `db:"owner_user_id"`
	OwnerOrgID           string            `db:"owner_org_id"`
	ConfidentialityLevel int32             `db:"confidentiality_level"`
	Version              int32             `db:"version"`
	IsLocked             bool              `db:"is_locked"`
	LockedAt             *time.Time        `db:"locked_at"`
	LockedBy             string            `db:"locked_by"`
	RetentionUntil       string            `db:"retention_until"`
	SortFinal            string            `db:"sort_final"`
	Metadata             map[string]string `db:"metadata"`
}

// AuditEvent is an append-only probative audit record.
type AuditEvent struct {
	ID            uuid.UUID      `db:"id"`
	SubjectID     uuid.UUID      `db:"subject_id"`
	EventType     string         `db:"event_type"`
	ActorUserID   string         `db:"actor_user_id"`
	OccurredAt    time.Time      `db:"occurred_at"`
	BeforeState   map[string]any `db:"before_state"`
	AfterState    map[string]any `db:"after_state"`
	Reason        string         `db:"reason"`
	CorrelationID *uuid.UUID     `db:"correlation_id"`
	RequestID     string         `db:"request_id"`
	Metadata      map[string]any `db:"metadata"`
}

// RelationshipType is an allowed typed relation between two subject kinds.
type RelationshipType struct {
	ID           uuid.UUID   `db:"id"`
	Code         string      `db:"code"`
	Label        string      `db:"label"`
	SourceKind   SubjectKind `db:"source_kind"`
	TargetKind   SubjectKind `db:"target_kind"`
	IsDirected   bool        `db:"is_directed"`
	InverseLabel string      `db:"inverse_label"`
	Description  string      `db:"description"`
	IsActive     bool        `db:"is_active"`
}

// SubjectRelationship is an actual typed edge between two subjects. The related
// SubjectRef and RelationshipType are hydrated by the repository for read paths.
type SubjectRelationship struct {
	ID                 uuid.UUID  `db:"id"`
	SourceSubjectID    uuid.UUID  `db:"source_subject_id"`
	TargetSubjectID    uuid.UUID  `db:"target_subject_id"`
	RelationshipTypeID uuid.UUID  `db:"relationship_type_id"`
	RoleDetail         string     `db:"role_detail"`
	ValidFrom          *time.Time `db:"valid_from"`
	ValidTo            *time.Time `db:"valid_to"`
	CreatedAt          time.Time  `db:"created_at"`
	CreatedBy          string     `db:"created_by"`
	DeletedAt          *time.Time `db:"deleted_at"`

	// Hydrated associations (nil on write paths).
	Source           *SubjectRef       `db:"-"`
	Target           *SubjectRef       `db:"-"`
	RelationshipType *RelationshipType `db:"-"`
}

// CreateSubjectInput holds the client-controlled fields for a new subject + its governance record.
type CreateSubjectInput struct {
	Kind                 SubjectKind
	DisplayLabel         string
	CanonicalURL         string
	OperatorID           string
	OwnerUserID          string
	OwnerOrgID           string
	ConfidentialityLevel int32
	RetentionUntil       string
	SortFinal            string
	Metadata             map[string]string
}

// LinkInput holds the fields required to create a typed relationship.
type LinkInput struct {
	SourceSubjectID      uuid.UUID
	TargetSubjectID      uuid.UUID
	RelationshipTypeCode string
	RoleDetail           string
	OperatorID           string
	ValidFrom            *time.Time
}

// RelationshipFilter controls relationship listing for one subject.
type RelationshipFilter struct {
	SubjectID            uuid.UUID
	Outgoing             bool
	RelationshipTypeCode string
	Limit                int
	Offset               int
}

// RelationshipResult holds a page of relationships and the total count before pagination.
type RelationshipResult struct {
	Relationships []*SubjectRelationship
	TotalSize     int32
}

// AuditFilter controls audit-event listing.
type AuditFilter struct {
	SubjectID uuid.UUID
	EventType string
	From      *time.Time
	To        *time.Time
	Limit     int
	Offset    int
}

// AuditResult holds a page of audit events and the total count before pagination.
type AuditResult struct {
	Events    []*AuditEvent
	TotalSize int32
}
