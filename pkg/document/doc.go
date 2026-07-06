// Package document implements the Document component of the Goéland POC: a
// first-class, probative, relational document entity (a modern GED slice).
//
// A document is a subject: document.id is a subject_ref.id of kind DOCUMENT
// (pinned by a composite foreign key). The package composes the transversal
// primitives from package core (subject_ref, record_metadata, audit_event,
// relationships) so that creating a document also creates its identity,
// governance record and audit trail atomically. It adds document-specific,
// probative and search-oriented concerns: cryptographic integrity (sha256),
// no-duplication external references, versioning, records-management flags,
// controlled classification, and full-text search over a generated tsvector.
package document
