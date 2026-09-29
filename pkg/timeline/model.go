package timeline

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EntryType mirrors the case_timeline_entry.entry_type column and the
// TimelineEntryType proto enum.
type EntryType int16

// Persisted values are 1 to 8 (CHECK constraint); 0 exists only in the API.
const (
	// TypeUnspecified is the proto zero value; never persisted, "any" in filters.
	TypeUnspecified EntryType = 0
	// TypeComment is a free follow-up note.
	TypeComment EntryType = 1
	// TypeOpinion is an opinion (préavis) given on the case.
	TypeOpinion EntryType = 2
	// TypeDecision is a business decision.
	TypeDecision EntryType = 3
	// TypeRequest is a request sent or received.
	TypeRequest EntryType = 4
	// TypeResponse is a response to a request.
	TypeResponse EntryType = 5
	// TypeValidation records a business validation act.
	TypeValidation EntryType = 6
	// TypeSystem is a fact written by the server; always locked.
	TypeSystem EntryType = 7
	// TypeAIProposal is a proposal awaiting human validation (GLD-030).
	TypeAIProposal EntryType = 8
)

// Valid reports whether t is a persisted entry type.
func (t EntryType) Valid() bool { return t >= TypeComment && t <= TypeAIProposal }

// OperatorCreatable reports whether an operator may create or keep an entry of
// type t through the API: SYSTEM entries are written by the server only and
// AI proposals are reserved for the AI component (GLD-030).
func (t EntryType) OperatorCreatable() bool {
	return t.Valid() && t != TypeSystem && t != TypeAIProposal
}

// Status mirrors the case_timeline_entry.status column and the
// TimelineEntryStatus proto enum.
type Status int16

// Persisted values are 1 to 4 (CHECK constraint); 0 exists only in the API.
const (
	// StatusUnspecified is the proto zero value; never persisted.
	StatusUnspecified Status = 0
	// StatusDraft is an editable entry, the only mutable state.
	StatusDraft Status = 1
	// StatusValidated is an entry endorsed by an operator; immutable.
	StatusValidated Status = 2
	// StatusLocked is an entry frozen as is; immutable.
	StatusLocked Status = 3
	// StatusWithdrawn is a draft set aside; immutable and hidden by default.
	StatusWithdrawn Status = 4
)

// Correctable reports whether an entry in status s may be corrected by a new
// entry: only validated or locked entries (a draft is simply edited).
func (s Status) Correctable() bool { return s == StatusValidated || s == StatusLocked }

// Visibility mirrors the case_timeline_entry.visibility column and the
// TimelineVisibility proto enum. It is stored but not enforced before GLD-017.
type Visibility int16

// Persisted values are 1 to 3 (CHECK constraint); 0 means the default.
const (
	// VisibilityUnspecified is the proto zero value; normalized to VisibilityCaseParticipants.
	VisibilityUnspecified Visibility = 0
	// VisibilityCaseParticipants is visible to everyone taking part in the case.
	VisibilityCaseParticipants Visibility = 1
	// VisibilityInternal is visible to the administration only.
	VisibilityInternal Visibility = 2
	// VisibilityRestricted is visible to a restricted circle only.
	VisibilityRestricted Visibility = 3
)

// Valid reports whether v is a persisted visibility.
func (v Visibility) Valid() bool {
	return v >= VisibilityCaseParticipants && v <= VisibilityRestricted
}

// Entry is one element of a case timeline (a row of case_timeline_entry).
type Entry struct {
	// ID is the entry identity.
	ID uuid.UUID `db:"id"`
	// CaseID is the case the entry belongs to.
	CaseID uuid.UUID `db:"case_id"`
	// Type is the business nature of the entry.
	Type EntryType `db:"entry_type"`
	// Status is the lifecycle state; only StatusDraft changes.
	Status Status `db:"status"`
	// Title is an optional headline, at most MaxTitleLength code points.
	Title string `db:"title"`
	// Body is the non-blank content, at most MaxBodyLength code points.
	Body string `db:"body"`
	// Visibility is the intended audience.
	Visibility Visibility `db:"visibility"`
	// OccurredAt is the business date of the event; the timeline is ordered on it.
	OccurredAt time.Time `db:"occurred_at"`
	// CorrectsEntryID is the immutable entry of the same case this one corrects.
	CorrectsEntryID *uuid.UUID `db:"corrects_entry_id"`
	// Metadata is secondary JSONB data; for a SYSTEM entry, the structured event
	// behind it (see SystemEntry).
	Metadata map[string]any `db:"metadata"`
	// CorrectedByEntryID is the live (not withdrawn) entry correcting this one,
	// computed on read.
	CorrectedByEntryID *uuid.UUID `db:"corrected_by_entry_id"`
	// CreatedAt is the recording time.
	CreatedAt time.Time `db:"created_at"`
	// CreatedBy is the operator who recorded the entry (or triggered a SYSTEM entry).
	CreatedBy string `db:"created_by"`
	// UpdatedAt is the last draft change; nil when never changed.
	UpdatedAt *time.Time `db:"updated_at"`
	// UpdatedBy is the operator of the last draft change.
	UpdatedBy string `db:"updated_by"`
	// ValidatedAt is set exactly when Status is StatusValidated.
	ValidatedAt *time.Time `db:"validated_at"`
	// ValidatedBy is the operator who validated the entry.
	ValidatedBy string `db:"validated_by"`
	// LockedAt is set exactly when Status is StatusLocked.
	LockedAt *time.Time `db:"locked_at"`
	// LockedBy is the operator who locked the entry.
	LockedBy string `db:"locked_by"`
	// WithdrawnAt is set exactly when Status is StatusWithdrawn.
	WithdrawnAt *time.Time `db:"withdrawn_at"`
	// WithdrawnBy is the operator who withdrew the draft.
	WithdrawnBy string `db:"withdrawn_by"`
	// WithdrawalReason is the non-blank justification of a withdrawal.
	WithdrawalReason string `db:"withdrawal_reason"`

	// Documents are the live document links, hydrated on read paths.
	Documents []*DocumentLink `db:"-"`
}

// DocumentLink is a document cited by an entry (a live row of
// timeline_document_link, with the document label and pinned version number).
type DocumentLink struct {
	// ID is the link identity.
	ID uuid.UUID `db:"id"`
	// EntryID is the citing entry.
	EntryID uuid.UUID `db:"timeline_entry_id"`
	// DocumentID is the cited logical document.
	DocumentID uuid.UUID `db:"document_id"`
	// DocumentLabel is the document's subject display label.
	DocumentLabel string `db:"document_label"`
	// DocumentVersionID is the version pinned when the entry left the draft
	// state; nil for a draft or a document without version.
	DocumentVersionID *uuid.UUID `db:"document_version_id"`
	// DocumentVersionNo is the pinned version number; nil when none is pinned.
	DocumentVersionNo *int32 `db:"version_no"`
	// CreatedAt is when the document was linked.
	CreatedAt time.Time `db:"created_at"`
	// CreatedBy is the operator who linked the document.
	CreatedBy string `db:"created_by"`
}

// CreateInput holds the fields of a new draft entry.
type CreateInput struct {
	// CaseID is the open case receiving the entry.
	CaseID uuid.UUID
	// Type must be operator-creatable (see EntryType.OperatorCreatable).
	Type EntryType
	// Title is an optional headline; surrounding whitespace is trimmed.
	Title string
	// Body is required; surrounding whitespace is trimmed.
	Body string
	// Visibility defaults to VisibilityCaseParticipants.
	Visibility Visibility
	// OccurredAt is the business date; nil means now, never in the future.
	OccurredAt *time.Time
	// CorrectsEntryID optionally names a validated or locked entry of the same
	// case to correct; an entry is corrected at most once.
	CorrectsEntryID *uuid.UUID
	// DocumentIDs are documents to cite; each one is linked to the case when it
	// is not already.
	DocumentIDs []uuid.UUID
	// OperatorID is the authenticated caller, set server-side.
	OperatorID string
}

// UpdateInput replaces the content of a draft.
type UpdateInput struct {
	// Type must be operator-creatable.
	Type EntryType
	// Title replaces the headline.
	Title string
	// Body replaces the content; required.
	Body string
	// Visibility replaces the audience; unspecified means VisibilityCaseParticipants.
	Visibility Visibility
	// OccurredAt replaces the business date when non-nil.
	OccurredAt *time.Time
	// OperatorID is the authenticated caller, set server-side.
	OperatorID string
	// Reason is the justification recorded on the audit event.
	Reason string
}

// SystemEntry is an entry the server writes in a case timeline, born locked
// (see RecordSystemEntryTx): a fact (SYSTEM) or a statement recorded on an
// operator's behalf by another component (e.g. a circulation RESPONSE).
type SystemEntry struct {
	// Type is the entry type; zero means TypeSystem.
	Type EntryType
	// CaseID is the case the fact belongs to.
	CaseID uuid.UUID
	// Title is the headline, e.g. "Changement de statut".
	Title string
	// Body is the readable text of the fact; required.
	Body string
	// Metadata describes the fact in a structured way; its "event" key names it
	// (e.g. CASE_STATUS_CHANGED) so clients can render it in their language.
	Metadata map[string]any
	// OperatorID is the operator whose action produced the fact.
	OperatorID string
}

// ListFilter selects the entries of one case, most recent OccurredAt first.
type ListFilter struct {
	// CaseID is the case whose timeline is listed.
	CaseID uuid.UUID
	// Types restricts the result to these types; empty means any.
	Types []EntryType
	// IncludeWithdrawn also returns withdrawn drafts.
	IncludeWithdrawn bool
	// Limit is the page size, normalized to [1, core.MaxPageSize].
	Limit int
	// Offset is the zero-based number of rows to skip; negative becomes 0.
	Offset int
}

// ListResult holds a page of entries and the total count before pagination.
type ListResult struct {
	// Entries is the requested page.
	Entries []*Entry
	// TotalSize is the number of matching entries across all pages.
	TotalSize int32
	// DraftCount is the number of drafts of the case, whatever the filters.
	DraftCount int32
}

// typeNames are the stable names used in audit events and errors (they match
// the TimelineEntryType proto enum without its prefix).
var typeNames = map[EntryType]string{
	TypeUnspecified: "UNSPECIFIED",
	TypeComment:     "COMMENT",
	TypeOpinion:     "OPINION",
	TypeDecision:    "DECISION",
	TypeRequest:     "REQUEST",
	TypeResponse:    "RESPONSE",
	TypeValidation:  "VALIDATION",
	TypeSystem:      "SYSTEM",
	TypeAIProposal:  "AI_PROPOSAL",
}

// String returns the stable type name (e.g. DECISION).
func (t EntryType) String() string {
	if name, ok := typeNames[t]; ok {
		return name
	}
	return fmt.Sprintf("EntryType(%d)", int16(t))
}

// statusNames are the stable status names (TimelineEntryStatus without prefix).
var statusNames = map[Status]string{
	StatusUnspecified: "UNSPECIFIED",
	StatusDraft:       "DRAFT",
	StatusValidated:   "VALIDATED",
	StatusLocked:      "LOCKED",
	StatusWithdrawn:   "WITHDRAWN",
}

// String returns the stable status name (e.g. VALIDATED).
func (s Status) String() string {
	if name, ok := statusNames[s]; ok {
		return name
	}
	return fmt.Sprintf("Status(%d)", int16(s))
}
