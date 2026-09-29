// Package circulation implements the Goéland POC case circulations (spec v2
// §29): sending a case to several recipients for their answer, as a light
// orchestration of tasks.
//
// Each recipient — one internal user or one live org unit — gets a task
// (origin CIRCULATION, managed by the circulation) when its step opens.
// Recipients of one step answer in parallel; the next step opens once the
// previous one has fully answered. Answering completes the recipient's task and
// records a locked RESPONSE entry in the case timeline; the last answer
// completes the circulation with a SYSTEM summary entry. Cancelling a
// circulation cancels its open tasks.
//
// A circulation belongs to its case and is audited on the CASE subject; a
// closed case freezes it, and the case lifecycle calls
// EnsureNoOpenCirculationsTx before closing a case. Overdue is computed from
// the deadline: late answers are accepted (automatic expiry comes with GLD-028).
package circulation
