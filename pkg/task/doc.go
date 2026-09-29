// Package task implements the Goéland POC case tasks (spec v2 §28): work to do
// on a case, independent of any workflow.
//
// A task belongs to its case (it is not a subject); its audit events are
// written on the CASE subject in the same transaction as the change. It has a
// type from an administrable catalogue, an optional deadline, and a small
// state machine (see Move): OPEN → IN_PROGRESS → DONE, or CANCELLED with a
// reason, and a done or cancelled task may be reopened with a reason.
//
// A task is assigned to one internal user or one live org unit, or to nobody
// yet; every (re)assignment is kept in case_task_assignment. "My tasks" are
// the tasks assigned to the caller or, when asked, to an org unit the caller
// belongs to (USER_MEMBER_OF_ORG_UNIT relationships).
//
// Completing or cancelling a task records a SYSTEM entry in the case timeline.
// A closed case freezes its tasks, and the case lifecycle (package casefile)
// calls EnsureNoOpenTasksTx before closing a case. Origin records what created
// a task: MANUAL today, CIRCULATION, WORKFLOW and AI for later slices.
package task
