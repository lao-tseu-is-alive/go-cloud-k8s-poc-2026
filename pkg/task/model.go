package task

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Status mirrors the case_task.status column and the TaskStatus proto enum.
type Status int16

// Persisted values are 1 to 4 (CHECK constraint); 0 exists only in the API.
const (
	// StatusUnspecified is the proto zero value; never persisted, "any" in filters.
	StatusUnspecified Status = 0
	// StatusOpen is a task to do.
	StatusOpen Status = 1
	// StatusInProgress is a task being worked on.
	StatusInProgress Status = 2
	// StatusDone is a completed task.
	StatusDone Status = 3
	// StatusCancelled is a task that will not be done.
	StatusCancelled Status = 4
)

// Valid reports whether s is a persisted status.
func (s Status) Valid() bool { return s >= StatusOpen && s <= StatusCancelled }

// Pending reports whether a task in status s still has to be done (open or in
// progress): only a pending task is edited, assigned, completed or cancelled.
func (s Status) Pending() bool { return s == StatusOpen || s == StatusInProgress }

// Origin mirrors the case_task.origin column and the TaskOrigin proto enum.
type Origin int16

const (
	// OriginManual is a task created by an operator (the only one creatable today).
	OriginManual Origin = 1
	// OriginCirculation is a task created by a circulation (GLD-013).
	OriginCirculation Origin = 2
	// OriginWorkflow is a task created by a workflow (GLD-031).
	OriginWorkflow Origin = 3
	// OriginAI is a task proposed by the AI component and validated (GLD-030).
	OriginAI Origin = 4
)

// TaskType is a controlled classification of tasks (administrable).
type TaskType struct {
	// ID is the catalogue row identity.
	ID uuid.UUID `db:"id"`
	// Code is the unique, non-blank stable key.
	Code string `db:"code"`
	// Label is the human label.
	Label string `db:"label"`
	// Description documents the business meaning of the type.
	Description string `db:"description"`
	// IsActive reports whether the type is offered for new tasks.
	IsActive bool `db:"is_active"`
}

// Assignee is who a task is assigned to: one internal user, one org unit, or
// nobody (both nil).
type Assignee struct {
	// UserID is the operator id of an internal user (app_user).
	UserID *string
	// OrgUnitID is a live org unit.
	OrgUnitID *uuid.UUID
}

// None reports an unassigned task.
func (a Assignee) None() bool { return a.UserID == nil && a.OrgUnitID == nil }

// Task is a row of case_task with its computed labels.
type Task struct {
	// ID is the task identity.
	ID uuid.UUID `db:"id"`
	// CaseID is the case the task belongs to.
	CaseID uuid.UUID `db:"case_id"`
	// CaseLabel is the case's business reference and label, computed on read.
	CaseLabel string `db:"case_label"`
	// TypeID references the task's TaskType.
	TypeID uuid.UUID `db:"task_type_id"`
	// Title is the non-blank title, at most MaxTitleLength code points.
	Title string `db:"title"`
	// Description details the work, at most MaxDescriptionLength code points.
	Description string `db:"description"`
	// Status is the lifecycle state.
	Status Status `db:"status"`
	// Origin is what created the task.
	Origin Origin `db:"origin"`
	// OriginRef names the creating object; empty for a manual task.
	OriginRef string `db:"origin_ref"`
	// AssigneeUserID is the assigned internal user; nil otherwise.
	AssigneeUserID *string `db:"assignee_user_id"`
	// AssigneeOrgUnitID is the assigned org unit; nil otherwise.
	AssigneeOrgUnitID *uuid.UUID `db:"assignee_org_unit_id"`
	// AssigneeLabel is the assignee's display name, computed on read.
	AssigneeLabel string `db:"assignee_label"`
	// DueAt is the deadline; nil when none.
	DueAt *time.Time `db:"due_at"`
	// CreatedAt is the creation time.
	CreatedAt time.Time `db:"created_at"`
	// CreatedBy is the operator who created the task.
	CreatedBy string `db:"created_by"`
	// UpdatedAt is maintained by a database trigger.
	UpdatedAt time.Time `db:"updated_at"`
	// UpdatedBy is the operator of the last change.
	UpdatedBy string `db:"updated_by"`
	// StartedAt is when the task was (last) started.
	StartedAt *time.Time `db:"started_at"`
	// StartedBy is the operator who started the task.
	StartedBy string `db:"started_by"`
	// CompletedAt is set exactly while the task is done.
	CompletedAt *time.Time `db:"completed_at"`
	// CompletedBy is the operator who completed the task.
	CompletedBy string `db:"completed_by"`
	// CompletionNote is the note left when completing.
	CompletionNote string `db:"completion_note"`
	// CancelledAt is set exactly while the task is cancelled.
	CancelledAt *time.Time `db:"cancelled_at"`
	// CancelledBy is the operator who cancelled the task.
	CancelledBy string `db:"cancelled_by"`
	// CancellationReason is the non-blank justification of a cancellation.
	CancellationReason string `db:"cancellation_reason"`

	// Type is the hydrated task type on read paths.
	Type *TaskType `db:"-"`
	// Assignments is the assignment history, oldest first (GetTask only).
	Assignments []*Assignment `db:"-"`
}

// Overdue reports a pending task past its deadline at now.
func (t *Task) Overdue(now time.Time) bool {
	return t.Status.Pending() && t.DueAt != nil && t.DueAt.Before(now)
}

// Assignment is one (re)assignment of a task (a row of case_task_assignment).
type Assignment struct {
	// ID is the assignment identity.
	ID uuid.UUID `db:"id"`
	// TaskID is the assigned task.
	TaskID uuid.UUID `db:"task_id"`
	// AssigneeUserID is the assigned internal user; nil when a unit is assigned.
	AssigneeUserID *string `db:"assignee_user_id"`
	// AssigneeOrgUnitID is the assigned org unit; nil when a user is assigned.
	AssigneeOrgUnitID *uuid.UUID `db:"assignee_org_unit_id"`
	// AssigneeLabel is the assignee's display name, computed on read.
	AssigneeLabel string `db:"assignee_label"`
	// AssignedAt is when the assignment started.
	AssignedAt time.Time `db:"assigned_at"`
	// AssignedBy is the operator who assigned the task.
	AssignedBy string `db:"assigned_by"`
	// Reason is the justification of the (re)assignment.
	Reason string `db:"reason"`
	// EndedAt is when the assignment ended; nil for the current one.
	EndedAt *time.Time `db:"ended_at"`
}

// Content is the operator-editable content shared by create and update.
type Content struct {
	// TypeCode is the code of a task type (active, unless it is the current one).
	TypeCode string
	// Title is required; surrounding whitespace is trimmed.
	Title string
	// Description is optional.
	Description string
	// DueAt is the optional deadline.
	DueAt *time.Time
}

// CreateInput is a new manual task.
type CreateInput struct {
	Content
	// CaseID is the open case receiving the task.
	CaseID uuid.UUID
	// Assignee is the initial assignee; the zero value leaves the task unassigned.
	Assignee Assignee
	// OperatorID is the authenticated caller, set server-side.
	OperatorID string
}

// UpdateInput replaces the content of a pending task.
type UpdateInput struct {
	Content
	// OperatorID is the authenticated caller, set server-side.
	OperatorID string
	// Reason is the justification recorded on the audit event.
	Reason string
}

// CaseFilter selects the tasks of one case.
type CaseFilter struct {
	// CaseID is the case whose tasks are listed.
	CaseID uuid.UUID
	// Statuses restricts the result; empty means every status.
	Statuses []Status
	// Limit is the page size, normalized to [1, core.MaxPageSize].
	Limit int
	// Offset is the zero-based number of rows to skip; negative becomes 0.
	Offset int
}

// MineFilter selects the caller's tasks across cases.
type MineFilter struct {
	// UserID is the caller's operator id, set server-side.
	UserID string
	// IncludeUnits also returns the tasks of the caller's org units.
	IncludeUnits bool
	// Statuses restricts the result; empty means the pending statuses.
	Statuses []Status
	// Limit is the page size, normalized to [1, core.MaxPageSize].
	Limit int
	// Offset is the zero-based number of rows to skip; negative becomes 0.
	Offset int
}

// ListResult is a page of tasks with the total count before pagination.
type ListResult struct {
	// Tasks is the requested page.
	Tasks []*Task
	// TotalSize is the number of matching tasks across all pages.
	TotalSize int32
	// OpenCount is the number of pending tasks of the case (case lists only).
	OpenCount int32
}

// statusNames are the stable status names (TaskStatus without prefix).
var statusNames = map[Status]string{
	StatusUnspecified: "UNSPECIFIED",
	StatusOpen:        "OPEN",
	StatusInProgress:  "IN_PROGRESS",
	StatusDone:        "DONE",
	StatusCancelled:   "CANCELLED",
}

// String returns the stable status name (e.g. IN_PROGRESS).
func (s Status) String() string {
	if name, ok := statusNames[s]; ok {
		return name
	}
	return fmt.Sprintf("Status(%d)", int16(s))
}
