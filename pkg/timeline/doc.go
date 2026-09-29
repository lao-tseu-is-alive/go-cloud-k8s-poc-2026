// Package timeline implements the Goéland POC case timeline (suivis, spec v2
// §26-27): the chronological business history of a case — comments, opinions,
// decisions, requests, responses, validations and server-written facts.
//
// An entry belongs to its case and is not a subject of its own: its audit
// events are written on the CASE subject (with the entry id in metadata) in
// the same transaction as the mutation. Only a draft changes; a validated or
// locked entry is immutable (also enforced by a database trigger) and a
// correction is a new entry naming the entry it corrects. A draft set aside is
// withdrawn, never deleted. A closed case accepts no timeline change.
//
// Documents are cited by their logical identity; the version current when the
// entry is validated or locked is pinned so a decision keeps pointing at the
// exact probative version. Citing a document also links it to the case.
//
// The case lifecycle (package casefile) uses EnsureNoDraftsTx before closing a
// case and RecordSystemEntryTx to record status changes; this package never
// imports casefile, so the dependency stays one-way.
package timeline
