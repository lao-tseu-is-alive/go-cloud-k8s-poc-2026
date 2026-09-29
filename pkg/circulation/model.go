package circulation

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/task"
)

// Status mirrors the case_circulation.status column and the CirculationStatus proto enum.
type Status int16

const (
	// StatusOpen is a circulation waiting for responses.
	StatusOpen Status = 1
	// StatusCompleted is a circulation every recipient answered.
	StatusCompleted Status = 2
	// StatusCancelled is a circulation stopped with a reason.
	StatusCancelled Status = 3
)

// Response mirrors case_circulation_recipient.response and the CirculationResponse proto enum.
type Response int16

// Persisted values are 1 to 5 (CHECK constraint); 0 means "not answered".
const (
	// ResponseNone is the zero value: not answered yet.
	ResponseNone Response = 0
	// ResponseFavorable is a favourable opinion.
	ResponseFavorable Response = 1
	// ResponseUnfavorable is an unfavourable opinion.
	ResponseUnfavorable Response = 2
	// ResponseComment is a remark without an opinion.
	ResponseComment Response = 3
	// ResponseNotConcerned means the recipient is not concerned.
	ResponseNotConcerned Response = 4
	// ResponseNeedMoreInfo asks for more information.
	ResponseNeedMoreInfo Response = 5
)

// Valid reports whether r is a persisted response.
func (r Response) Valid() bool { return r >= ResponseFavorable && r <= ResponseNeedMoreInfo }

// NeedsText reports a response that is meaningless without a text.
func (r Response) NeedsText() bool { return r == ResponseComment || r == ResponseNeedMoreInfo }

// labelsFR are the French labels written in the timeline entries (clients may
// render the structured metadata in their language instead).
var labelsFR = map[Response]string{
	ResponseFavorable:    "Favorable",
	ResponseUnfavorable:  "Défavorable",
	ResponseComment:      "Remarque",
	ResponseNotConcerned: "Non concerné",
	ResponseNeedMoreInfo: "Complément d'information demandé",
}

// names are the stable response names (CirculationResponse without prefix).
var names = map[Response]string{
	ResponseNone:         "NONE",
	ResponseFavorable:    "FAVORABLE",
	ResponseUnfavorable:  "UNFAVORABLE",
	ResponseComment:      "COMMENT",
	ResponseNotConcerned: "NOT_CONCERNED",
	ResponseNeedMoreInfo: "NEED_MORE_INFO",
}

// String returns the stable response name (e.g. FAVORABLE).
func (r Response) String() string {
	if name, ok := names[r]; ok {
		return name
	}
	return fmt.Sprintf("Response(%d)", int16(r))
}

// Circulation is a row of case_circulation with its recipients.
type Circulation struct {
	// ID is the circulation identity.
	ID uuid.UUID `db:"id"`
	// CaseID is the case the circulation belongs to.
	CaseID uuid.UUID `db:"case_id"`
	// Title names the circulation.
	Title string `db:"title"`
	// Message is the request sent to the recipients.
	Message string `db:"message"`
	// DueAt is the deadline of the answers; nil when none.
	DueAt *time.Time `db:"due_at"`
	// Status is the lifecycle state.
	Status Status `db:"status"`
	// CurrentStep is the open step (the last one once completed).
	CurrentStep int32 `db:"current_step"`
	// StepCount is the number of steps.
	StepCount int32 `db:"step_count"`
	// CreatedAt is the creation time.
	CreatedAt time.Time `db:"created_at"`
	// CreatedBy is the operator who sent the circulation.
	CreatedBy string `db:"created_by"`
	// UpdatedAt is maintained by a database trigger.
	UpdatedAt time.Time `db:"updated_at"`
	// CompletedAt is set once every recipient answered.
	CompletedAt *time.Time `db:"completed_at"`
	// CancelledAt is set when the circulation was cancelled.
	CancelledAt *time.Time `db:"cancelled_at"`
	// CancelledBy is the operator who cancelled it.
	CancelledBy string `db:"cancelled_by"`
	// CancellationReason is the justification of the cancellation.
	CancellationReason string `db:"cancellation_reason"`

	// Recipients are hydrated on read paths, by step then label.
	Recipients []*Recipient `db:"-"`
}

// Overdue reports an open circulation past its deadline at now.
func (c *Circulation) Overdue(now time.Time) bool {
	return c.Status == StatusOpen && c.DueAt != nil && c.DueAt.Before(now)
}

// Recipient is a row of case_circulation_recipient with its labels.
type Recipient struct {
	// ID is the recipient identity.
	ID uuid.UUID `db:"id"`
	// CirculationID is the circulation.
	CirculationID uuid.UUID `db:"circulation_id"`
	// Step is the recipient's step (1 first).
	Step int32 `db:"step"`
	// AssigneeUserID is the internal user; nil when a unit is the recipient.
	AssigneeUserID *string `db:"assignee_user_id"`
	// AssigneeOrgUnitID is the org unit; nil when a user is the recipient.
	AssigneeOrgUnitID *uuid.UUID `db:"assignee_org_unit_id"`
	// AssigneeLabel is the recipient's display name, computed on read.
	AssigneeLabel string `db:"assignee_label"`
	// TaskID is the recipient's task; nil until its step opens.
	TaskID *uuid.UUID `db:"task_id"`
	// TaskStatus is the status of that task, computed on read.
	TaskStatus *task.Status `db:"task_status"`
	// Response is the answer; nil until given.
	Response *Response `db:"response"`
	// ResponseText is the text of the answer.
	ResponseText string `db:"response_text"`
	// RespondedAt is when the answer was recorded.
	RespondedAt *time.Time `db:"responded_at"`
	// RespondedBy is the operator who recorded the answer.
	RespondedBy string `db:"responded_by"`
	// ResponseEntryID is the RESPONSE timeline entry of the answer.
	ResponseEntryID *uuid.UUID `db:"response_entry_id"`
}

// Awaiting reports a recipient whose step is open in an open circulation and
// who has not answered yet.
func (r *Recipient) Awaiting(c *Circulation) bool {
	return c.Status == StatusOpen && r.Step == c.CurrentStep && r.Response == nil
}

// RecipientInput is a recipient of a new circulation.
type RecipientInput struct {
	// Step is the recipient's step; 0 means 1. Steps are renumbered from 1 in order.
	Step int32
	// Assignee is exactly one internal user or org unit.
	Assignee task.Assignee
}

// CreateInput is a new circulation.
type CreateInput struct {
	// CaseID is the open case sent to the recipients.
	CaseID uuid.UUID
	// Title names the circulation; required.
	Title string
	// Message is the request sent to the recipients.
	Message string
	// DueAt is the optional deadline of the answers.
	DueAt *time.Time
	// Recipients are 1 to MaxRecipients distinct users or units.
	Recipients []RecipientInput
	// OperatorID is the authenticated caller, set server-side.
	OperatorID string
}

// RespondInput is the answer of a recipient.
type RespondInput struct {
	// RecipientID is the answering recipient.
	RecipientID uuid.UUID
	// Response is the answer.
	Response Response
	// Text details the answer; required for COMMENT and NEED_MORE_INFO.
	Text string
	// OperatorID is the authenticated caller recording the answer, set server-side.
	OperatorID string
}
