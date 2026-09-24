package casefile

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Status mirrors the case_file.status column and the CaseStatus proto enum.
type Status int16

// Persisted values are 1 to 4 (CHECK constraint); 0 exists only in the API.
const (
	// StatusUnspecified is the proto zero value; never persisted, "any" in searches.
	StatusUnspecified Status = 0
	// StatusOpen is a newly opened or reopened case.
	StatusOpen Status = 1
	// StatusInProgress is a case being actively processed.
	StatusInProgress Status = 2
	// StatusSuspended is a case put on hold.
	StatusSuspended Status = 3
	// StatusClosed is an operationally finished case; its metadata is frozen
	// until it is reopened.
	StatusClosed Status = 4
)

// Valid reports whether s is a persisted status.
func (s Status) Valid() bool { return s >= StatusOpen && s <= StatusClosed }

// transitions lists, per current status, the statuses a case may move to.
var transitions = map[Status][]Status{
	StatusOpen:       {StatusInProgress, StatusSuspended, StatusClosed},
	StatusInProgress: {StatusOpen, StatusSuspended, StatusClosed},
	StatusSuspended:  {StatusOpen, StatusInProgress, StatusClosed},
	StatusClosed:     {StatusOpen},
}

// CanTransition reports whether a case in status from may move to status to.
// Staying in the same status is not a transition.
func CanTransition(from, to Status) bool {
	for _, allowed := range transitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

// RequiresReason reports whether the transition from → to must be justified:
// closing a case and reopening a closed one.
func RequiresReason(from, to Status) bool {
	return to == StatusClosed || from == StatusClosed
}

// CaseType is a controlled classification of cases. Types are seeded
// reference data; a case's type is fixed at creation.
type CaseType struct {
	// ID is the catalogue row identity.
	ID uuid.UUID `db:"id"`
	// Code is the unique, non-blank stable key used by APIs (e.g. OPC_DEMANDE_PC).
	Code string `db:"code"`
	// Label is the human label.
	Label string `db:"label"`
	// Description documents the business meaning of the type.
	Description string `db:"description"`
	// BusinessRefNamespace, when non-empty, is the namespace in which CreateCase
	// allocates the case's business reference by default.
	BusinessRefNamespace string `db:"business_ref_namespace"`
	// IsActive reports whether the type is offered for new cases.
	IsActive bool `db:"is_active"`
}

// Case is the case-specific projection (1:1 with a CASE subject_ref). The
// generated search_vector column is deliberately absent: the application never
// writes or scans it.
type Case struct {
	// ID is the CASE subject_ref ID; a composite foreign key pins the kind.
	ID uuid.UUID `db:"id"`
	// CaseTypeID references the CaseType chosen at creation.
	CaseTypeID uuid.UUID `db:"case_type_id"`
	// Title is the non-blank title, at most MaxTitleLength code points; it is
	// mirrored into the subject's display label.
	Title string `db:"title"`
	// Description is free text, at most MaxDescriptionLength code points.
	Description string `db:"description"`
	// Status is the operational state.
	Status Status `db:"status"`
	// OpenedAt is when the case was opened.
	OpenedAt time.Time `db:"opened_at"`
	// ClosedAt is when the case was closed; nil unless Status is StatusClosed.
	ClosedAt *time.Time `db:"closed_at"`
	// ClosedBy is the operator who closed the case; empty unless closed.
	ClosedBy string `db:"closed_by"`
	// ClosureReason is the justification given when closing; empty unless closed.
	ClosureReason string `db:"closure_reason"`
	// Metadata is secondary JSONB data; never holds critical fields.
	Metadata map[string]any `db:"metadata"`
	// CreatedAt is the database insertion time.
	CreatedAt time.Time `db:"created_at"`
	// CreatedBy is the operator who created the case.
	CreatedBy string `db:"created_by"`
	// UpdatedAt is maintained by a database trigger on every update.
	UpdatedAt time.Time `db:"updated_at"`

	// Subject is the hydrated identity (with business reference) on read paths.
	Subject *core.SubjectRef `db:"-"`
	// RecordMetadata is the hydrated governance record on read paths.
	RecordMetadata *core.RecordMetadata `db:"-"`
	// Type is the hydrated case type on read paths.
	Type *CaseType `db:"-"`
}

// CreateInput holds the client-controlled fields for a new case. Service.Create
// persists the subject, governance record, business reference, case and audit
// event in one transaction.
type CreateInput struct {
	// CaseTypeCode is the required code of an active case type.
	CaseTypeCode string
	// Title is required; surrounding whitespace is trimmed.
	Title string
	// Description is optional free text.
	Description string
	// Metadata is secondary extension data.
	Metadata map[string]any
	// BusinessRef overrides the business reference; the zero value allocates
	// one in the case type's namespace when it has one, otherwise assigns none.
	BusinessRef core.BusinessRefRequest
	// OperatorID is the authenticated caller, set server-side; it becomes
	// created_by, the default owner and the audit actor.
	OperatorID string
	// Governance carries the requested owner, confidentiality and records
	// fields; the service overwrites its identity fields from the case.
	Governance core.CreateSubjectInput
}

// UpdateInput holds the editable metadata of a case. Every field replaces the
// stored value; a closed, locked or deleted case is rejected.
type UpdateInput struct {
	// Title is required; surrounding whitespace is trimmed.
	Title string
	// Description replaces the description.
	Description string
	// Metadata replaces the whole extension map.
	Metadata map[string]any
	// OperatorID is the authenticated caller, set server-side.
	OperatorID string
	// Reason is the justification recorded on the audit event.
	Reason string
}

// TransitionInput requests a status change (see CanTransition).
type TransitionInput struct {
	// Target is the requested status.
	Target Status
	// OperatorID is the authenticated caller, set server-side.
	OperatorID string
	// Reason is the justification; required when RequiresReason.
	Reason string
}

// SearchFilter controls case search. Results are newest first.
type SearchFilter struct {
	// Query is matched accent-insensitively against title and description, or
	// exactly against the business reference; empty matches every case.
	Query string
	// CaseTypeCode restricts results to one type; empty means any.
	CaseTypeCode string
	// Status restricts results to one status; StatusUnspecified means any.
	Status Status
	// IncludeDeleted also returns soft-deleted cases.
	IncludeDeleted bool
	// Limit is the page size, normalized to [1, core.MaxPageSize].
	Limit int
	// Offset is the zero-based number of rows to skip; negative becomes 0.
	Offset int
}

// SearchResult holds a page of cases and the total count before pagination.
type SearchResult struct {
	// Cases is the requested page.
	Cases []*Case
	// TotalSize is the number of matching cases across all pages.
	TotalSize int32
}

// statusNames are the stable names used in audit events and errors (they
// match the CaseStatus proto enum without its CASE_STATUS_ prefix).
var statusNames = map[Status]string{
	StatusUnspecified: "UNSPECIFIED",
	StatusOpen:        "OPEN",
	StatusInProgress:  "IN_PROGRESS",
	StatusSuspended:   "SUSPENDED",
	StatusClosed:      "CLOSED",
}

// String returns the stable status name (e.g. IN_PROGRESS).
func (s Status) String() string {
	if name, ok := statusNames[s]; ok {
		return name
	}
	return fmt.Sprintf("Status(%d)", int16(s))
}
