package task

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/timeline"
)

// Move is a lifecycle change of a task.
type Move int

const (
	// MoveStart is OPEN → IN_PROGRESS.
	MoveStart Move = iota + 1
	// MoveComplete is OPEN | IN_PROGRESS → DONE (optional note).
	MoveComplete
	// MoveCancel is OPEN | IN_PROGRESS → CANCELLED (required reason).
	MoveCancel
	// MoveReopen is DONE | CANCELLED → OPEN (required reason).
	MoveReopen
)

// moveRule describes one Move: the statuses it starts from, its SQL, its audit
// event and, for completion and cancellation, its SYSTEM timeline entry.
type moveRule struct {
	from          []Status
	sql           string
	event         string
	past          string
	timelineTitle string
	noteLabel     string
}

// allowed reports whether the move may start from status s.
func (m moveRule) allowed(s Status) bool { return slices.Contains(m.from, s) }

// moves is the task state machine.
var moves = map[Move]moveRule{
	MoveStart:    {from: []Status{StatusOpen}, sql: startTaskSQL, event: "TASK_STARTED", past: "started"},
	MoveComplete: {from: []Status{StatusOpen, StatusInProgress}, sql: completeTaskSQL, event: "TASK_COMPLETED", past: "completed", timelineTitle: "Tâche terminée", noteLabel: "Note"},
	MoveCancel:   {from: []Status{StatusOpen, StatusInProgress}, sql: cancelTaskSQL, event: "TASK_CANCELLED", past: "cancelled", timelineTitle: "Tâche annulée", noteLabel: "Motif"},
	MoveReopen:   {from: []Status{StatusDone, StatusCancelled}, sql: reopenTaskSQL, event: "TASK_REOPENED", past: "reopened"},
}

// MoveOptions tunes MoveTx for the component that created a task.
type MoveOptions struct {
	// ByOrigin is set by the component a non-manual task belongs to (e.g. the
	// circulation): only it may complete, cancel or reopen such a task.
	ByOrigin bool
	// SkipTimeline leaves out the SYSTEM timeline entry, when the caller writes
	// its own (a circulation response, a circulation cancellation).
	SkipTimeline bool
}

// CreateTx inserts a task in the caller's transaction: the case must be open,
// the type active and the assignee valid; it records the first assignment and
// writes TASK_CREATED on the case. origin and originRef name what created the
// task (OriginManual and "" for an operator).
func CreateTx(ctx context.Context, q core.Querier, in CreateInput, origin Origin, originRef string) (*Task, *core.AuditEvent, error) {
	if err := core.EnsureOpenCaseTx(ctx, q, in.CaseID); err != nil {
		return nil, nil, err
	}
	// A manual task needs MANAGE on the case; a task created by another
	// component (circulation, ...) is authorized by that component.
	if origin == OriginManual {
		if err := core.EnsureAccessTx(ctx, q, in.OperatorID, in.CaseID, core.LevelManage); err != nil {
			return nil, nil, err
		}
	}
	taskType, err := activeType(ctx, q, in.TypeCode)
	if err != nil {
		return nil, nil, err
	}
	if err := CheckAssigneeTx(ctx, q, in.Assignee); err != nil {
		return nil, nil, err
	}
	t, err := collectTask(q.Query(ctx, insertTaskSQL, pgx.NamedArgs{
		"case_id":              in.CaseID,
		"task_type_id":         taskType.ID,
		"title":                in.Title,
		"description":          in.Description,
		"due_at":               in.DueAt,
		"assignee_user_id":     in.Assignee.UserID,
		"assignee_org_unit_id": in.Assignee.OrgUnitID,
		"origin":               int16(origin),
		"origin_ref":           originRef,
		"operator_id":          in.OperatorID,
	}))
	if err != nil {
		return nil, nil, err
	}
	if err := recordAssignmentTx(ctx, q, t.ID, in.Assignee, in.OperatorID, ""); err != nil {
		return nil, nil, err
	}
	ev, err := auditTx(ctx, q, t, "TASK_CREATED", in.OperatorID, "", nil, taskState(t, taskType.Code))
	if err != nil {
		return nil, nil, err
	}
	return t, ev, nil
}

// MoveTx applies one lifecycle move in the caller's transaction and writes its
// audit event; completion and cancellation also record a SYSTEM timeline entry
// unless opts.SkipTimeline. A task created by another component (circulation,
// workflow, AI) can only be started directly: its other moves belong to that
// component (opts.ByOrigin).
func MoveTx(ctx context.Context, q core.Querier, id uuid.UUID, move Move, operatorID, note string, opts MoveOptions) (*Task, *core.AuditEvent, error) {
	rule, known := moves[move]
	if !known {
		return nil, nil, fmt.Errorf("%w: unknown task move %d", core.ErrInvalidInput, move)
	}
	current, err := lockTaskTx(ctx, q, id)
	if err != nil {
		return nil, nil, err
	}
	if err := ensureMoveAccessTx(ctx, q, operatorID, current, move); err != nil {
		return nil, nil, err
	}
	if move != MoveStart && !opts.ByOrigin {
		if err := ensureManualTx(current); err != nil {
			return nil, nil, err
		}
	}
	if !rule.allowed(current.Status) {
		return nil, nil, fmt.Errorf("%w: a %s task cannot be %s", core.ErrInvalidState, current.Status, rule.past)
	}
	t, err := collectTask(q.Query(ctx, rule.sql, pgx.NamedArgs{"id": id, "operator_id": operatorID, "note": note}))
	if err != nil {
		return nil, nil, err
	}
	ev, err := auditTx(ctx, q, t, rule.event, operatorID, note,
		map[string]any{"status": current.Status.String()}, map[string]any{"status": t.Status.String()})
	if err != nil || rule.timelineTitle == "" || opts.SkipTimeline {
		return t, ev, err
	}
	if _, err := timeline.RecordSystemEntryTx(ctx, q, systemEntry(t, rule, operatorID, note)); err != nil {
		return nil, nil, err
	}
	return t, ev, nil
}

// ensureMoveAccessTx requires the level of a move: starting or completing is
// the assignee's own work (or CONTRIBUTE on the case), cancelling and
// reopening need MANAGE.
func ensureMoveAccessTx(ctx context.Context, q core.Querier, operatorID string, t *Task, move Move) error {
	if move == MoveStart || move == MoveComplete {
		return core.EnsureAssigneeOrAccessTx(ctx, q, operatorID, t.CaseID, t.AssigneeUserID, t.AssigneeOrgUnitID, core.LevelContribute)
	}
	return core.EnsureAccessTx(ctx, q, operatorID, t.CaseID, core.LevelManage)
}

// ensureManualTx refuses a direct change to a task another component manages.
func ensureManualTx(t *Task) error {
	if t.Origin != OriginManual {
		return fmt.Errorf("%w: this task is managed by the object that created it (e.g. answer the circulation)", core.ErrInvalidState)
	}
	return nil
}

// EnsureNoOpenTasksTx fails with core.ErrInvalidState when the case still has
// open or in-progress tasks. The case lifecycle calls it before closing a case.
func EnsureNoOpenTasksTx(ctx context.Context, q core.Querier, caseID uuid.UUID) error {
	pending, err := countPendingTx(ctx, q, caseID)
	if err != nil {
		return err
	}
	if pending > 0 {
		return fmt.Errorf("%w: the case has %d open tasks; complete or cancel them before closing it", core.ErrInvalidState, pending)
	}
	return nil
}

// countPendingTx counts the open or in-progress tasks of a case.
func countPendingTx(ctx context.Context, q core.Querier, caseID uuid.UUID) (int32, error) {
	var n int32
	if err := q.QueryRow(ctx, countPendingSQL, pgx.NamedArgs{"case_id": caseID}).Scan(&n); err != nil {
		return 0, fmt.Errorf("count open tasks: %w", err)
	}
	return n, nil
}

// lockTaskTx locks the task's case (core.EnsureOpenCaseTx: a closed, locked or
// deleted case refuses every change), then the task row. The case is locked
// first, as on creation, to avoid lock inversions.
func lockTaskTx(ctx context.Context, q core.Querier, id uuid.UUID) (*Task, error) {
	current, err := collectTask(q.Query(ctx, getTaskSQL, pgx.NamedArgs{"id": id}))
	if err != nil {
		return nil, err
	}
	if err := core.EnsureOpenCaseTx(ctx, q, current.CaseID); err != nil {
		return nil, err
	}
	return collectTask(q.Query(ctx, getTaskForUpdateSQL, pgx.NamedArgs{"id": id}))
}

// lockPendingTx is lockTaskTx for a change that needs a pending task.
func lockPendingTx(ctx context.Context, q core.Querier, id uuid.UUID) (*Task, error) {
	t, err := lockTaskTx(ctx, q, id)
	if err != nil {
		return nil, err
	}
	if !t.Status.Pending() {
		return nil, fmt.Errorf("%w: a %s task must be reopened before it changes", core.ErrInvalidState, t.Status)
	}
	return t, nil
}

// lockManualPendingTx is lockPendingTx for a manual task the operator manages
// (MANAGE on its case): an edit or a reassignment.
func lockManualPendingTx(ctx context.Context, q core.Querier, id uuid.UUID, operatorID string) (*Task, error) {
	t, err := lockPendingTx(ctx, q, id)
	if err != nil {
		return nil, err
	}
	if err := core.EnsureAccessTx(ctx, q, operatorID, t.CaseID, core.LevelManage); err != nil {
		return nil, err
	}
	return t, ensureManualTx(t)
}

// CheckAssigneeTx requires at most one assignee, an existing internal user or
// a live org unit (core.ErrInvalidInput otherwise).
func CheckAssigneeTx(ctx context.Context, q core.Querier, a Assignee) error {
	switch {
	case a.UserID != nil && a.OrgUnitID != nil:
		return fmt.Errorf("%w: assign a user or an org unit, not both", core.ErrInvalidInput)
	case a.OrgUnitID != nil:
		return core.EnsureLiveOrgUnitTx(ctx, q, *a.OrgUnitID)
	case a.UserID != nil:
		var known bool
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM app_user WHERE user_id = $1)`, *a.UserID).Scan(&known); err != nil {
			return fmt.Errorf("check assignee: %w", err)
		}
		if !known {
			return fmt.Errorf("%w: unknown internal user %q", core.ErrInvalidInput, *a.UserID)
		}
	}
	return nil
}

// recordAssignmentTx appends the current assignment to the history (nothing
// for an unassigned task).
func recordAssignmentTx(ctx context.Context, q core.Querier, taskID uuid.UUID, a Assignee, operatorID, reason string) error {
	if a.None() {
		return nil
	}
	_, err := q.Exec(ctx, insertAssignmentSQL, pgx.NamedArgs{
		"task_id": taskID, "assignee_user_id": a.UserID, "assignee_org_unit_id": a.OrgUnitID,
		"operator_id": operatorID, "reason": reason,
	})
	if err != nil {
		return fmt.Errorf("record assignment: %w", err)
	}
	return nil
}

// sameAssignee reports whether a matches the task's current assignee.
func sameAssignee(t *Task, a Assignee) bool {
	return equalPtr(t.AssigneeUserID, a.UserID) && equalPtr(t.AssigneeOrgUnitID, a.OrgUnitID)
}

// equalPtr compares two optional values.
func equalPtr[T comparable](x, y *T) bool {
	if x == nil || y == nil {
		return x == y
	}
	return *x == *y
}

// assigneeState is the audited assignee of a task.
func assigneeState(t *Task) map[string]any {
	state := map[string]any{"assignee_user_id": "", "assignee_org_unit_id": ""}
	if t.AssigneeUserID != nil {
		state["assignee_user_id"] = *t.AssigneeUserID
	}
	if t.AssigneeOrgUnitID != nil {
		state["assignee_org_unit_id"] = t.AssigneeOrgUnitID.String()
	}
	return state
}

// taskState is the audited snapshot of a task.
func taskState(t *Task, typeCode string) map[string]any {
	state := assigneeState(t)
	state["title"], state["type"], state["status"] = t.Title, typeCode, t.Status.String()
	if t.DueAt != nil {
		state["due_at"] = *t.DueAt
	}
	return state
}

// auditTx writes a task audit event on the task's CASE subject, naming the
// task in metadata so the case trail shows the whole task history.
func auditTx(ctx context.Context, q core.Querier, t *Task, eventType, operatorID, reason string, before, after map[string]any) (*core.AuditEvent, error) {
	ev, err := core.InsertAuditEventTx(ctx, q, core.AuditEvent{
		SubjectID: t.CaseID, EventType: eventType, ActorUserID: operatorID, Reason: reason,
		BeforeState: before, AfterState: after,
		Metadata: map[string]any{"task_id": t.ID.String()},
	})
	if err != nil {
		return nil, fmt.Errorf("insert audit_event: %w", err)
	}
	return ev, nil
}

// systemEntry describes a completion or cancellation as a SYSTEM timeline
// entry: a French readable body and the structured event for clients.
func systemEntry(t *Task, rule moveRule, operatorID, note string) timeline.SystemEntry {
	body := rule.timelineTitle + " : " + t.Title
	if note != "" {
		body += "\n" + rule.noteLabel + " : " + note
	}
	return timeline.SystemEntry{
		CaseID: t.CaseID,
		Title:  rule.timelineTitle,
		Body:   body,
		Metadata: map[string]any{
			"event": rule.event, "task_id": t.ID.String(), "title": t.Title, "note": note,
		},
		OperatorID: operatorID,
	}
}

// resolveType returns a task's current type and the requested one: the
// current type stays valid even when deactivated, a new type must be active.
func resolveType(ctx context.Context, q core.Querier, currentID uuid.UUID, code string) (*TaskType, *TaskType, error) {
	currentType, err := getType(ctx, q, getTypeByIDSQL, pgx.NamedArgs{"id": currentID})
	if err != nil {
		return nil, nil, err
	}
	if code == currentType.Code {
		return currentType, currentType, nil
	}
	taskType, err := activeType(ctx, q, code)
	return currentType, taskType, err
}

// activeType loads a task type by code and requires it to be active.
func activeType(ctx context.Context, q core.Querier, code string) (*TaskType, error) {
	taskType, err := getType(ctx, q, getTypeByCodeSQL, pgx.NamedArgs{"code": code})
	if err != nil {
		return nil, fmt.Errorf("task type %q: %w", code, err)
	}
	if !taskType.IsActive {
		return nil, fmt.Errorf("%w: task type %q is inactive", core.ErrInvalidInput, code)
	}
	return taskType, nil
}

// getType loads one task type with the given query and arguments.
func getType(ctx context.Context, q core.Querier, sql string, args pgx.NamedArgs) (*TaskType, error) {
	rows, err := q.Query(ctx, sql, args)
	if err != nil {
		return nil, fmt.Errorf("query task type: %w", err)
	}
	t, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[TaskType])
	if err != nil {
		return nil, mapDBError(err)
	}
	return t, nil
}

// collectTask reads exactly one task row from a query result.
func collectTask(rows pgx.Rows, err error) (*Task, error) {
	if err != nil {
		return nil, fmt.Errorf("query task: %w", err)
	}
	t, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Task])
	if err != nil {
		return nil, mapDBError(err)
	}
	return t, nil
}
