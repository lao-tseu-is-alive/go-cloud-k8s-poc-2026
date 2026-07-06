// Package core implements the transversal domain of the Goéland POC: the elements
// common to every business subject.
//
// It owns the canonical identity (SubjectRef), governance (RecordMetadata:
// ownership, confidentiality, non-destructive lifecycle, locking), the append-only
// audit log (AuditEvent), and the typed relationship graph (RelationshipType /
// SubjectRelationship). Sibling domains such as document reuse the exported
// transaction-scoped helpers (InsertSubjectRefTx, InsertRecordMetadataTx,
// InsertAuditEventTx, LinkSubjectsTx, ...) so that an entity and its identity,
// governance and audit trail are created atomically in a single transaction.
package core
