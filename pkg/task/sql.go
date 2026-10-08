package task

import (
	"strings"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// SQL fragments for the task repository. Column projections are the single
// source of truth for pgx named scanning (columns map to `db` tags); they are
// alias-prefixed, so INSERT statements alias their target (AS t).

const rawTaskColumns = `
t.id, t.case_id, t.task_type_id, t.title, t.description, t.status, t.origin, t.origin_ref,
t.assignee_user_id, t.assignee_org_unit_id, t.due_at, t.created_at, t.created_by,
t.updated_at, t.updated_by, t.started_at, t.started_by, t.completed_at, t.completed_by,
t.completion_note, t.cancelled_at, t.cancelled_by, t.cancellation_reason`

// assigneeLabelExpr names the assignee of a row aliased x, joined with its
// app_user (au) and the unit's subject_ref (su): a unit reads as its subject
// label, a user as its name, e-mail or id. It serves tasks (x → t) and
// assignments.
const assigneeLabelExpr = `
CASE WHEN x.assignee_user_id IS NOT NULL
     THEN coalesce(nullif(au.display_name, ''), nullif(au.email, ''), x.assignee_user_id)
     ELSE coalesce(su.display_label, '') END`

// AssigneeLabelSQL is the SQL expression naming the assignee of a row aliased
// alias (with assignee_user_id and assignee_org_unit_id columns); the query
// must LEFT JOIN app_user au ON the user and subject_ref su ON the unit.
func AssigneeLabelSQL(alias string) string {
	return strings.ReplaceAll(assigneeLabelExpr, "x.", alias+".")
}

// readTaskColumns adds the case and assignee labels (needs taskFrom).
var readTaskColumns = rawTaskColumns + `,
CASE WHEN sr.business_ref <> '' THEN sr.business_ref || ' — ' || sr.display_label ELSE sr.display_label END AS case_label,
` + AssigneeLabelSQL("t") + ` AS assignee_label`

const taskFrom = `
FROM case_task t
JOIN subject_ref sr ON sr.id = t.case_id
LEFT JOIN app_user au ON au.user_id = t.assignee_user_id
LEFT JOIN subject_ref su ON su.id = t.assignee_org_unit_id`

// pendingFirst orders pending tasks first, then by deadline (none last).
const pendingFirst = `
ORDER BY (t.status IN (1, 2)) DESC, t.due_at ASC NULLS LAST, t.created_at, t.id`

const insertTaskSQL = `
INSERT INTO case_task AS t (case_id, task_type_id, title, description, due_at,
    assignee_user_id, assignee_org_unit_id, origin, origin_ref, created_by, updated_by)
VALUES (@case_id, @task_type_id, @title, @description, @due_at,
    @assignee_user_id, @assignee_org_unit_id, @origin, @origin_ref, @operator_id, @operator_id)
RETURNING ` + rawTaskColumns + `;`

var getTaskSQL = `
SELECT ` + readTaskColumns + taskFrom + `
WHERE t.id = @id;`

// getTaskForUpdateSQL locks the task row for a check-then-mutate on its status.
const getTaskForUpdateSQL = `
SELECT ` + rawTaskColumns + `
FROM case_task t
WHERE t.id = @id
FOR UPDATE;`

const updateTaskSQL = `
UPDATE case_task t
SET task_type_id = @task_type_id, title = @title, description = @description,
    due_at = @due_at, updated_by = @operator_id
WHERE t.id = @id
RETURNING ` + rawTaskColumns + `;`

const setAssigneeSQL = `
UPDATE case_task t
SET assignee_user_id = @assignee_user_id, assignee_org_unit_id = @assignee_org_unit_id,
    updated_by = @operator_id
WHERE t.id = @id
RETURNING ` + rawTaskColumns + `;`

const startTaskSQL = `
UPDATE case_task t
SET status = 2, started_at = now(), started_by = @operator_id, updated_by = @operator_id
WHERE t.id = @id
RETURNING ` + rawTaskColumns + `;`

const completeTaskSQL = `
UPDATE case_task t
SET status = 3, completed_at = now(), completed_by = @operator_id, completion_note = @note,
    updated_by = @operator_id
WHERE t.id = @id
RETURNING ` + rawTaskColumns + `;`

const cancelTaskSQL = `
UPDATE case_task t
SET status = 4, cancelled_at = now(), cancelled_by = @operator_id, cancellation_reason = @note,
    updated_by = @operator_id
WHERE t.id = @id
RETURNING ` + rawTaskColumns + `;`

// reopenTaskSQL puts a done or cancelled task back to OPEN, clearing the
// completion and cancellation stamps (they stay in the audit trail).
const reopenTaskSQL = `
UPDATE case_task t
SET status = 1, completed_at = NULL, completed_by = '', completion_note = '',
    cancelled_at = NULL, cancelled_by = '', cancellation_reason = '', updated_by = @operator_id
WHERE t.id = @id
RETURNING ` + rawTaskColumns + `;`

// taskSortFields are the sortable columns of the task lists (GLD-055): the
// case and assignee labels are output columns of readTaskColumns.
var taskSortFields = map[string]core.SortField{
	"title":    {Expr: "t.title"},
	"case":     {Expr: "case_label"},
	"assignee": {Expr: "assignee_label", Nullable: true},
	"due_at":   {Expr: "t.due_at", Nullable: true},
	"status":   {Expr: "t.status"},
}

// taskOrders holds the ORDER BY clause of each sort; the zero Sort is the
// list's own default order.
func taskOrders(defaultOrder string) map[core.Sort]string {
	orders := core.SortedQueries(taskSortFields, func(f core.SortField, desc bool) string {
		return "\nORDER BY " + f.Expr + core.OrderDirection(f.Order(desc)) + ", t.id"
	})
	orders[core.Sort{}] = defaultOrder
	return orders
}

// listCaseTasksSQL holds, per sort, the page of the tasks of a case (pending first by default).
var listCaseTasksSQL = sortedTaskQueries(`
SELECT `+readTaskColumns+`,
COUNT(*) OVER() AS total_count`+taskFrom+`
WHERE t.case_id = @case_id
  AND (cardinality(@statuses::smallint[]) = 0 OR t.status = ANY(@statuses::smallint[]))`, pendingFirst)

// sortedTaskQueries appends each sort's ORDER BY and the page to query.
func sortedTaskQueries(query, defaultOrder string) map[core.Sort]string {
	out := map[core.Sort]string{}
	for sort, order := range taskOrders(defaultOrder) {
		out[sort] = query + order + "\nLIMIT @limit OFFSET @offset;"
	}
	return out
}

const countPendingSQL = `
SELECT count(*) FROM case_task
WHERE case_id = @case_id AND status IN (1, 2);`

// listMyTasksSQL holds, per sort, the tasks of live cases assigned to the user
// or, when asked, to a unit the user belongs to (open USER_MEMBER_OF_ORG_UNIT
// edge), earliest deadline first by default.
var listMyTasksSQL = sortedTaskQueries(`
SELECT `+readTaskColumns+`,
COUNT(*) OVER() AS total_count`+taskFrom+`
JOIN record_metadata rm ON rm.subject_id = t.case_id
WHERE rm.deleted_at IS NULL
  AND t.status = ANY(@statuses::smallint[])
  AND (t.assignee_user_id = @user_id
       OR (@include_units AND t.assignee_org_unit_id IN (
           SELECT r.target_subject_id
           FROM subject_relationship r
           JOIN relationship_type rt ON rt.id = r.relationship_type_id
           JOIN app_user me ON me.subject_id = r.source_subject_id
           WHERE rt.code = 'USER_MEMBER_OF_ORG_UNIT' AND me.user_id = @user_id
             AND r.deleted_at IS NULL AND r.valid_to IS NULL)))`, `
ORDER BY t.due_at ASC NULLS LAST, t.created_at, t.id`)

// --- assignments --------------------------------------------------------------------

const endCurrentAssignmentSQL = `
UPDATE case_task_assignment
SET ended_at = now()
WHERE task_id = @task_id AND ended_at IS NULL;`

const insertAssignmentSQL = `
INSERT INTO case_task_assignment (task_id, assignee_user_id, assignee_org_unit_id, assigned_by, reason)
VALUES (@task_id, @assignee_user_id, @assignee_org_unit_id, @operator_id, @reason);`

var listAssignmentsSQL = `
SELECT x.id, x.task_id, x.assignee_user_id, x.assignee_org_unit_id,
       ` + AssigneeLabelSQL("x") + ` AS assignee_label,
       x.assigned_at, x.assigned_by, x.reason, x.ended_at
FROM case_task_assignment x
LEFT JOIN app_user au ON au.user_id = x.assignee_user_id
LEFT JOIN subject_ref su ON su.id = x.assignee_org_unit_id
WHERE x.task_id = @task_id
ORDER BY x.assigned_at, x.id;`

// --- task_type ----------------------------------------------------------------------

const typeColumns = `id, code, label, description, is_active`

const getTypeByCodeSQL = `
SELECT ` + typeColumns + `
FROM task_type
WHERE code = @code;`

const getTypeByIDSQL = `
SELECT ` + typeColumns + `
FROM task_type
WHERE id = @id;`

const listTypesSQL = `
SELECT ` + typeColumns + `
FROM task_type
WHERE (NOT @only_active OR is_active = true)
ORDER BY label, code;`

const insertTypeSQL = `
INSERT INTO task_type (code, label, description)
VALUES (@code, @label, @description)
RETURNING ` + typeColumns + `;`

const getTypeForUpdateSQL = `
SELECT ` + typeColumns + `
FROM task_type
WHERE code = @code
FOR UPDATE;`

// updateTypeSQL replaces the fields given (NULL keeps the current value).
const updateTypeSQL = `
UPDATE task_type
SET label = coalesce(@label::text, label),
    description = coalesce(@description::text, description),
    is_active = coalesce(@is_active::boolean, is_active)
WHERE code = @code
RETURNING ` + typeColumns + `;`
