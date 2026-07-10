// Package actor implements the Goéland POC Actor component: external persons
// (ACTOR_KIND_PERSON) and organizations (ACTOR_KIND_ORGANIZATION).
//
// An actor is a first-class subject (actor.id == subject_ref.id of kind ACTOR).
// Identity/governance/audit are reused from the core module (subject_ref,
// record_metadata, audit_event) so an actor and its governance are created
// atomically. Actors are wired to cases/documents/things exclusively through
// typed CoreService relationships (CASE_HAS_ACTOR_*, DOCUMENT_AUTHORED_BY_ACTOR,
// ...) — never through fields on the actor — so roles live in relationship_type.
//
// This first slice covers identity + typed contacts. Addresses and the full role
// vocabulary are later slices.
package actor
