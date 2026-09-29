package circulation

import "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/task"

// SQL fragments for the circulation repository. Column projections are the
// single source of truth for pgx named scanning (columns map to `db` tags);
// they are alias-prefixed, so INSERT statements alias their target (AS c).

const circulationColumns = `
c.id, c.case_id, c.title, c.message, c.due_at, c.status, c.current_step, c.step_count,
c.created_at, c.created_by, c.updated_at, c.completed_at, c.cancelled_at, c.cancelled_by,
c.cancellation_reason`

const insertCirculationSQL = `
INSERT INTO case_circulation AS c (case_id, title, message, due_at, step_count, created_by)
VALUES (@case_id, @title, @message, @due_at, @step_count, @operator_id)
RETURNING ` + circulationColumns + `;`

const getCirculationSQL = `
SELECT ` + circulationColumns + `
FROM case_circulation c
WHERE c.id = @id;`

// getCirculationForUpdateSQL locks the circulation for a check-then-mutate.
const getCirculationForUpdateSQL = `
SELECT ` + circulationColumns + `
FROM case_circulation c
WHERE c.id = @id
FOR UPDATE;`

const listCaseCirculationsSQL = `
SELECT ` + circulationColumns + `
FROM case_circulation c
WHERE c.case_id = @case_id
ORDER BY c.created_at DESC, c.id;`

const openNextStepSQL = `
UPDATE case_circulation c
SET current_step = current_step + 1
WHERE c.id = @id
RETURNING ` + circulationColumns + `;`

const completeCirculationSQL = `
UPDATE case_circulation c
SET status = 2, completed_at = now()
WHERE c.id = @id
RETURNING ` + circulationColumns + `;`

const cancelCirculationSQL = `
UPDATE case_circulation c
SET status = 3, cancelled_at = now(), cancelled_by = @operator_id, cancellation_reason = @reason
WHERE c.id = @id
RETURNING ` + circulationColumns + `;`

const countOpenCirculationsSQL = `
SELECT count(*) FROM case_circulation
WHERE case_id = @case_id AND status = 1;`

// --- recipients ---------------------------------------------------------------------

const rawRecipientColumns = `
r.id, r.circulation_id, r.step, r.assignee_user_id, r.assignee_org_unit_id, r.task_id,
r.response, r.response_text, r.responded_at, r.responded_by, r.response_entry_id`

const insertRecipientSQL = `
INSERT INTO case_circulation_recipient (circulation_id, step, assignee_user_id, assignee_org_unit_id)
VALUES (@circulation_id, @step, @assignee_user_id, @assignee_org_unit_id);`

// listRecipientsSQL returns the recipients of several circulations with their
// label and task status, by step then label.
var listRecipientsSQL = `
SELECT ` + rawRecipientColumns + `,
       ` + task.AssigneeLabelSQL("r") + ` AS assignee_label,
       ct.status AS task_status
FROM case_circulation_recipient r
LEFT JOIN app_user au ON au.user_id = r.assignee_user_id
LEFT JOIN subject_ref su ON su.id = r.assignee_org_unit_id
LEFT JOIN case_task ct ON ct.id = r.task_id
WHERE r.circulation_id = ANY(@ids::uuid[])
ORDER BY r.step, assignee_label, r.id;`

// getRecipientForUpdateSQL locks one recipient row.
const getRecipientForUpdateSQL = `
SELECT ` + rawRecipientColumns + `
FROM case_circulation_recipient r
WHERE r.id = @id
FOR UPDATE;`

// getRecipientSQL reads one recipient with its label.
var getRecipientSQL = `
SELECT ` + rawRecipientColumns + `,
       ` + task.AssigneeLabelSQL("r") + ` AS assignee_label
FROM case_circulation_recipient r
LEFT JOIN app_user au ON au.user_id = r.assignee_user_id
LEFT JOIN subject_ref su ON su.id = r.assignee_org_unit_id
WHERE r.id = @id;`

// stepRecipientsSQL returns the recipients of one step, with their label (for
// the tasks created when the step opens).
var stepRecipientsSQL = `
SELECT ` + rawRecipientColumns + `,
       ` + task.AssigneeLabelSQL("r") + ` AS assignee_label
FROM case_circulation_recipient r
LEFT JOIN app_user au ON au.user_id = r.assignee_user_id
LEFT JOIN subject_ref su ON su.id = r.assignee_org_unit_id
WHERE r.circulation_id = @circulation_id AND r.step = @step
ORDER BY r.id;`

const setRecipientTaskSQL = `
UPDATE case_circulation_recipient
SET task_id = @task_id
WHERE id = @id;`

const recordResponseSQL = `
UPDATE case_circulation_recipient
SET response = @response, response_text = @text, responded_at = now(),
    responded_by = @operator_id, response_entry_id = @entry_id
WHERE id = @id;`

const countUnansweredSQL = `
SELECT count(*) FROM case_circulation_recipient
WHERE circulation_id = @circulation_id AND step = @step AND response IS NULL;`

// responseCountsSQL counts the answers of a circulation by response.
const responseCountsSQL = `
SELECT response, count(*) AS n
FROM case_circulation_recipient
WHERE circulation_id = @circulation_id AND response IS NOT NULL
GROUP BY response
ORDER BY response;`

// pendingTasksSQL returns the open or in-progress tasks of a circulation.
const pendingTasksSQL = `
SELECT r.task_id
FROM case_circulation_recipient r
JOIN case_task ct ON ct.id = r.task_id
WHERE r.circulation_id = @circulation_id AND ct.status IN (1, 2);`
