package retention

import (
	"time"

	"github.com/google/uuid"
)

// Disposition bordereaux (GLD-032 step B): due cases of one final disposition
// gathered by a records manager, approved case by case, then executed.

// BordereauStatus is the state of a bordereau.
type BordereauStatus string

// The bordereau states (disposition_bordereau.status).
const (
	// BordereauOpen waits for its approvals, then for its execution.
	BordereauOpen BordereauStatus = "OPEN"
	// BordereauExecuted handled its approved cases; its proof is kept.
	BordereauExecuted BordereauStatus = "EXECUTED"
	// BordereauCancelled was abandoned before its execution.
	BordereauCancelled BordereauStatus = "CANCELLED"
)

// Decision is the approval of one case of a bordereau.
type Decision string

// The decisions (disposition_item.decision).
const (
	// DecisionPending waits for its approver.
	DecisionPending Decision = "PENDING"
	// DecisionApproved hands the case to the execution.
	DecisionApproved Decision = "APPROVED"
	// DecisionRejected leaves the case out of the execution.
	DecisionRejected Decision = "REJECTED"
)

// The executed outcomes of a retention (subject_retention.disposition_status).
const (
	// OutcomeArchived is a case transferred to the archives and frozen.
	OutcomeArchived = "ARCHIVED"
	// OutcomeDisposed is an eliminated case (step B2).
	OutcomeDisposed = "DISPOSED"
)

// Audit event types of the bordereaux, written on the CASE subject.
const (
	// EventDispositionProposed records a case put in a bordereau.
	EventDispositionProposed = "RETENTION_DISPOSITION_PROPOSED"
	// EventDispositionApproved records an approved case.
	EventDispositionApproved = "RETENTION_DISPOSITION_APPROVED"
	// EventDispositionRejected records a rejected case.
	EventDispositionRejected = "RETENTION_DISPOSITION_REJECTED"
	// EventDispositionWithdrawn records a case of a cancelled bordereau.
	EventDispositionWithdrawn = "RETENTION_DISPOSITION_WITHDRAWN"
	// EventArchived records a case transferred to the archives and frozen.
	EventArchived = "RETENTION_ARCHIVED"
	// EventDisposed records an eliminated case (its earlier events are blanked).
	EventDisposed = "RETENTION_DISPOSED"
	// EventTransferRebuilt records a transfer package queued again.
	EventTransferRebuilt = "RETENTION_TRANSFER_REBUILT"
)

// MaxBordereauCases bounds the cases of one bordereau.
const MaxBordereauCases = 500

// Bordereau gathers due cases of one final disposition.
type Bordereau struct {
	// ID identifies the bordereau.
	ID uuid.UUID `db:"id"`
	// FinalDisposition is ARCHIVE or DESTROY.
	FinalDisposition Disposition `db:"final_disposition"`
	// Status is the bordereau state.
	Status BordereauStatus `db:"status"`
	// Reason explains the bordereau.
	Reason string `db:"reason"`
	// PreparedBy is the records manager who prepared it.
	PreparedBy string `db:"prepared_by"`
	// PreparedAt is when it was prepared.
	PreparedAt time.Time `db:"prepared_at"`
	// ClosedAt is when it was executed or cancelled.
	ClosedAt *time.Time `db:"closed_at"`
	// ClosedBy is who executed or cancelled it.
	ClosedBy *string `db:"closed_by"`
	// CloseReason explains a cancellation.
	CloseReason string `db:"close_reason"`
	// ProofSHA256 is the SHA-256 of ProofManifest's bytes.
	ProofSHA256 string `db:"proof_sha256"`
	// ProofManifest is the proof manifest (JSON) exactly as hashed; empty until executed.
	ProofManifest string `db:"proof_manifest"`
	// ItemCount counts its cases.
	ItemCount int32 `db:"item_count"`
	// PendingCount counts its cases waiting for a decision.
	PendingCount int32 `db:"pending_count"`
	// ApprovedCount counts its approved cases.
	ApprovedCount int32 `db:"approved_count"`
	// TotalSize counts the matching bordereaux of a list (window count).
	TotalSize int32 `db:"total_size"`
	// Items are its cases (detail reads only).
	Items []*Item `db:"-"`
}

// Elimination is what the elimination of one approved case did, or would do
// when simulated (GLD-077).
type Elimination struct {
	// CaseID is the eliminated case.
	CaseID uuid.UUID
	// DocumentsEliminated counts the documents only the case held, blanked and deleted.
	DocumentsEliminated int
	// DocumentsKept counts the documents still linked to a thing or to a case that stays.
	DocumentsKept int
	// FilesDestroyed counts the files no other document version referenced.
	FilesDestroyed int
	// ExportsPurged counts the ordinary exports of the case removed.
	ExportsPurged int
}

// Item is one case of a bordereau with its approval.
type Item struct {
	// ID identifies the item.
	ID uuid.UUID `db:"id"`
	// BordereauID is its bordereau.
	BordereauID uuid.UUID `db:"bordereau_id"`
	// CaseID is the case.
	CaseID uuid.UUID `db:"case_id"`
	// RetentionID is the retention being executed.
	RetentionID uuid.UUID `db:"retention_id"`
	// CaseLabel is the case's label.
	CaseLabel string `db:"case_label"`
	// BusinessRef is the case's business reference.
	BusinessRef string `db:"business_ref"`
	// CaseTypeCode is the case's type.
	CaseTypeCode string `db:"case_type_code"`
	// UntilOn is the end of the case's retention.
	UntilOn time.Time `db:"until_on"`
	// StartsOn is the start of the case's retention.
	StartsOn time.Time `db:"starts_on"`
	// PolicyCode is the policy applied.
	PolicyCode string `db:"policy_code"`
	// ApproverSubjectID is the owning unit, or the case without one.
	ApproverSubjectID uuid.UUID `db:"approver_subject_id"`
	// ApproverLabel is the approver subject's label.
	ApproverLabel string `db:"approver_label"`
	// Decision is the approval state.
	Decision Decision `db:"decision"`
	// DecidedAt is when the approver decided.
	DecidedAt *time.Time `db:"decided_at"`
	// DecidedBy is the approver.
	DecidedBy *string `db:"decided_by"`
	// DecisionReason explains the decision.
	DecisionReason string `db:"decision_reason"`
	// ExportID is the transfer package of an executed ARCHIVE item.
	ExportID *uuid.UUID `db:"export_id"`
	// ExportStatus is that package's state.
	ExportStatus *string `db:"export_status"`
	// ExportSHA256 is that package's digest once built.
	ExportSHA256 *string `db:"export_sha256"`
}

// BordereauInput prepares a bordereau.
type BordereauInput struct {
	// Disposition is ARCHIVE or DESTROY.
	Disposition Disposition
	// Reason explains the bordereau.
	Reason string
	// CaseIDs are the due cases.
	CaseIDs []uuid.UUID
	// OperatorID is the records manager, set server-side.
	OperatorID string
}

// DecisionInput approves or rejects one case.
type DecisionInput struct {
	// ItemID is the item.
	ItemID uuid.UUID
	// Approve approves; false rejects.
	Approve bool
	// Reason explains the decision.
	Reason string
	// OperatorID is the approver, set server-side.
	OperatorID string
}

// BordereauFilter pages the bordereaux.
type BordereauFilter struct {
	// Status restricts the list; empty means all.
	Status BordereauStatus
	// Limit is the page size.
	Limit int
	// Offset is the page start.
	Offset int
}

// BordereauList is a page of bordereaux.
type BordereauList struct {
	// Bordereaux are the rows of the page.
	Bordereaux []*Bordereau
	// TotalSize counts every matching bordereau.
	TotalSize int32
}

// Approval is a pending item the caller may decide, with its bordereau's disposition.
type Approval struct {
	// Item is the pending item.
	Item *Item
	// Disposition is its bordereau's final disposition.
	Disposition Disposition
}

// proofManifest is the content whose SHA-256 proves an executed bordereau.
type proofManifest struct {
	// BordereauID is the bordereau.
	BordereauID uuid.UUID `json:"bordereau_id"`
	// FinalDisposition is ARCHIVE or DESTROY.
	FinalDisposition Disposition `json:"final_disposition"`
	// Reason explains the bordereau.
	Reason string `json:"reason"`
	// PreparedBy is the records manager who prepared it.
	PreparedBy string `json:"prepared_by"`
	// PreparedAt is when it was prepared.
	PreparedAt time.Time `json:"prepared_at"`
	// ExecutedBy is who executed it.
	ExecutedBy string `json:"executed_by"`
	// ExecutedAt is when it was executed.
	ExecutedAt time.Time `json:"executed_at"`
	// Items are its cases with their decisions.
	Items []proofItem `json:"items"`
}

// proofItem is one case of the proof manifest.
type proofItem struct {
	// CaseID is the case.
	CaseID uuid.UUID `json:"case_id"`
	// BusinessRef is its business reference.
	BusinessRef string `json:"business_ref"`
	// CaseTypeCode is its type.
	CaseTypeCode string `json:"case_type_code"`
	// PolicyCode is the policy applied.
	PolicyCode string `json:"policy_code"`
	// StartsOn is the start of its retention.
	StartsOn string `json:"starts_on"`
	// UntilOn is the end of its retention.
	UntilOn string `json:"until_on"`
	// Decision is the approval.
	Decision Decision `json:"decision"`
	// DecidedBy is the approver.
	DecidedBy string `json:"decided_by"`
	// DecidedAt is when it was decided.
	DecidedAt *time.Time `json:"decided_at"`
	// DecisionReason explains the decision.
	DecisionReason string `json:"decision_reason"`
	// ExportID is the transfer package of a transferred case.
	ExportID *uuid.UUID `json:"export_id,omitempty"`
}
