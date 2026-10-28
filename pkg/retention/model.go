package retention

import (
	"time"

	"github.com/google/uuid"
)

// Disposition is what happens to a case when its retention expires.
type Disposition string

// The final dispositions (retention_policy.final_disposition).
const (
	// DispositionArchive transfers the case to the archives.
	DispositionArchive Disposition = "ARCHIVE"
	// DispositionDestroy eliminates the case after an approved bordereau.
	DispositionDestroy Disposition = "DESTROY"
	// DispositionReview leaves the choice to the records manager.
	DispositionReview Disposition = "REVIEW"
)

// Valid reports whether d is a known disposition.
func (d Disposition) Valid() bool {
	return d == DispositionArchive || d == DispositionDestroy || d == DispositionReview
}

// Audit event types written on the CASE subject.
const (
	// EventRetentionStarted records a retention computed at closing or applied.
	EventRetentionStarted = "RETENTION_STARTED"
	// EventRetentionEnded records a retention ended by a reopening.
	EventRetentionEnded = "RETENTION_ENDED"
	// EventHoldPlaced records a legal hold placed on the case.
	EventHoldPlaced = "RETENTION_HOLD_PLACED"
	// EventHoldReleased records a legal hold released.
	EventHoldReleased = "RETENTION_HOLD_RELEASED"
)

// MaxDurationYears bounds a policy's duration (as the database CHECK).
const MaxDurationYears = 200

// MaxReasonLength bounds a hold or release reason (code points).
const MaxReasonLength = 2000

// MaxApplyBatch bounds the cases ApplyCaseType handles in one call (each one audited).
const MaxApplyBatch = 1000

// Policy is an entry of the retention schedule.
type Policy struct {
	// ID is the catalogue row id.
	ID uuid.UUID `db:"id"`
	// Code is the immutable key.
	Code string `db:"code"`
	// Label is the human label.
	Label string `db:"label"`
	// Description documents the policy.
	Description string `db:"description"`
	// LegalBasis cites the rule the policy applies.
	LegalBasis string `db:"legal_basis"`
	// DurationYears is how long a case is kept after its closing.
	DurationYears int16 `db:"duration_years"`
	// FinalDisposition is what happens when the retention expires.
	FinalDisposition Disposition `db:"final_disposition"`
	// IsActive is false for a policy that can no longer be given to a case type.
	IsActive bool `db:"is_active"`
}

// PolicyInput is a new policy (administrators only).
type PolicyInput struct {
	// Code is the immutable key (see core.ValidateReferenceCode).
	Code string
	// Label is the required human label.
	Label string
	// Description documents the policy.
	Description string
	// LegalBasis cites the rule the policy applies.
	LegalBasis string
	// DurationYears is how long a case is kept after its closing (0-200).
	DurationYears int32
	// FinalDisposition is what happens when the retention expires.
	FinalDisposition Disposition
	// OperatorID is the administrator, set server-side.
	OperatorID string
	// Reason is the justification recorded in the reference change log.
	Reason string
}

// PolicyUpdate changes a policy; nil fields are kept and the code is immutable.
type PolicyUpdate struct {
	// Label replaces the label when non-nil (not blank).
	Label *string
	// Description replaces the description when non-nil.
	Description *string
	// LegalBasis replaces the legal basis when non-nil.
	LegalBasis *string
	// DurationYears replaces the duration when non-nil (0-200).
	DurationYears *int32
	// FinalDisposition replaces the disposition when non-nil.
	FinalDisposition *Disposition
	// IsActive activates or deactivates the entry when non-nil.
	IsActive *bool
	// OperatorID is the administrator, set server-side.
	OperatorID string
	// Reason is the justification recorded in the reference change log.
	Reason string
}

// Retention is the retention of a closed case: a snapshot of its policy.
type Retention struct {
	// ID identifies the row.
	ID uuid.UUID `db:"id"`
	// SubjectID is the case.
	SubjectID uuid.UUID `db:"subject_id"`
	// PolicyCode is the policy applied.
	PolicyCode string `db:"policy_code"`
	// PolicyLabel is the policy's current label.
	PolicyLabel string `db:"policy_label"`
	// DurationYears is the duration applied.
	DurationYears int16 `db:"duration_years"`
	// FinalDisposition is the disposition applied.
	FinalDisposition Disposition `db:"final_disposition"`
	// StartsOn is the closing day (Europe/Zurich).
	StartsOn time.Time `db:"starts_on"`
	// UntilOn is the day the retention ends: the case is due from that day.
	UntilOn time.Time `db:"until_on"`
	// CreatedAt is when the retention was computed.
	CreatedAt time.Time `db:"created_at"`
	// CreatedBy is the operator who closed the case or applied the policy.
	CreatedBy string `db:"created_by"`
	// EndedAt is when the retention ended; nil while it runs.
	EndedAt *time.Time `db:"ended_at"`
	// EndedBy is the operator who ended it; nil while it runs.
	EndedBy *string `db:"ended_by"`
	// EndReason explains the end.
	EndReason string `db:"end_reason"`
	// DispositionStatus is the executed outcome (OutcomeArchived, OutcomeDisposed); nil while waiting.
	DispositionStatus *string `db:"disposition_status"`
	// DisposedAt is when the outcome was executed.
	DisposedAt *time.Time `db:"disposed_at"`
	// BordereauID is the bordereau that executed it.
	BordereauID *uuid.UUID `db:"bordereau_id"`
}

// State is the audited state of a retention.
func (r *Retention) State() map[string]any {
	return map[string]any{
		"policy_code": r.PolicyCode, "duration_years": r.DurationYears, "final_disposition": string(r.FinalDisposition),
		"starts_on": r.StartsOn.Format(time.DateOnly), "until_on": r.UntilOn.Format(time.DateOnly),
	}
}

// Hold is a legal hold on a case.
type Hold struct {
	// ID identifies the hold.
	ID uuid.UUID `db:"id"`
	// SubjectID is the case.
	SubjectID uuid.UUID `db:"subject_id"`
	// Reason explains the hold.
	Reason string `db:"reason"`
	// PlacedAt is when the hold was placed.
	PlacedAt time.Time `db:"placed_at"`
	// PlacedBy is the records manager who placed it.
	PlacedBy string `db:"placed_by"`
	// ReleasedAt is when the hold was released; nil while active.
	ReleasedAt *time.Time `db:"released_at"`
	// ReleasedBy is who released it; nil while active.
	ReleasedBy *string `db:"released_by"`
	// ReleaseReason explains the release.
	ReleaseReason string `db:"release_reason"`
}

// CaseRetention describes the retention of a case.
type CaseRetention struct {
	// Current is the running retention; nil while the case is open or without policy.
	Current *Retention
	// Hold is the active legal hold; nil when none.
	Hold *Hold
	// Retentions lists every retention of the case, newest first.
	Retentions []*Retention
	// Holds lists every hold of the case, newest first.
	Holds []*Hold
	// DefaultPolicy is the policy of the case's type; nil when none.
	DefaultPolicy *Policy
}

// HoldInput places or releases a hold.
type HoldInput struct {
	// CaseID is the case.
	CaseID uuid.UUID
	// Reason explains the change; it is audited.
	Reason string
	// OperatorID is the records manager, set server-side.
	OperatorID string
}

// DueFilter selects the running retentions ending on or before a day.
type DueFilter struct {
	// DueOn is the day (YYYY-MM-DD); empty means today in Europe/Zurich.
	DueOn string
	// Disposition restricts the list; empty means all.
	Disposition Disposition
	// IncludeHeld also lists the cases on legal hold.
	IncludeHeld bool
	// Limit is the page size.
	Limit int
	// Offset is the page start.
	Offset int
}

// DueRetention is a running retention of the due list, with its case.
type DueRetention struct {
	Retention
	// CaseLabel is the case's label.
	CaseLabel string `db:"case_label"`
	// BusinessRef is the case's business reference.
	BusinessRef string `db:"business_ref"`
	// CaseTypeCode is the case's type.
	CaseTypeCode string `db:"case_type_code"`
	// OnHold reports an active legal hold.
	OnHold bool `db:"on_hold"`
	// TotalSize counts every matching row (window count).
	TotalSize int32 `db:"total_size"`
}

// DueResult is a page of the due list.
type DueResult struct {
	// Retentions are the rows of the page.
	Retentions []*DueRetention
	// TotalSize counts every matching row.
	TotalSize int32
}

// ApplyInput gives a case type's policy to its closed cases without retention.
type ApplyInput struct {
	// CaseTypeCode is the case type.
	CaseTypeCode string
	// Reason is recorded on every case.
	Reason string
	// OperatorID is the administrator, set server-side.
	OperatorID string
}

// ApplyResult counts the cases handled by ApplyCaseType.
type ApplyResult struct {
	// Applied counts the cases given a retention.
	Applied int32
	// Remaining counts the closed cases of the type still without retention.
	Remaining int32
}
