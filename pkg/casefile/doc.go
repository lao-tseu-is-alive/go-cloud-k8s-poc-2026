// Package casefile implements the Goéland POC Case component (affaires, spec
// v1 §6.1, spec v2 §24): the administrative business file at the heart of the
// system.
//
// A case is a first-class subject (case_file.id == subject_ref.id of kind
// CASE). Identity, governance and audit reuse the core primitives, so a case,
// its governance record, its business reference and its audit event are
// created atomically. Participants, documents, things and related cases are
// typed CoreService relationships (CASE_HAS_ACTOR_*, CASE_HAS_DOCUMENT,
// CASE_CONCERNS_THING, CASE_PARENT_OF_CASE), never columns on the case.
//
// A case exists independently of any workflow; its operational lifecycle is a
// small, explicit state machine (see CanTransition). The package is named
// casefile because case is a Go keyword.
package casefile
