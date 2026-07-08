package core

// SQL fragments for the transversal core repository.
//
// Kept as raw SQL for full control. Column projections are the single source of
// truth for pgx named scanning (RowToStructByNameLax matches columns to `db` tags).
//
// Queries use pgx v5 named parameters (@name) bound through pgx.NamedArgs at the
// call sites, so the SQL reads self-documenting and the Go argument list can be
// checked against it by name rather than by position.

// --- subject_ref -------------------------------------------------------------

const subjectRefColumns = `id, kind, display_label, canonical_url, created_at`

const insertSubjectRefSQL = `
INSERT INTO subject_ref (kind, display_label, canonical_url)
VALUES (@kind, @display_label, @canonical_url)
RETURNING ` + subjectRefColumns + `;`

const getSubjectRefSQL = `
SELECT ` + subjectRefColumns + `
FROM subject_ref
WHERE id = @id;`

const getSubjectRefsByIDsSQL = `
SELECT ` + subjectRefColumns + `
FROM subject_ref
WHERE id = ANY(@ids::uuid[]);`

// updateSubjectRefLabelSQL keeps the canonical display_label in sync with a
// domain entity's human label (e.g. a document title) so graph projections
// never show a stale label.
const updateSubjectRefLabelSQL = `
UPDATE subject_ref SET display_label = @display_label WHERE id = @id;`

// --- record_metadata ---------------------------------------------------------

const recordMetadataColumns = `
subject_id, created_at, created_by, updated_at, updated_by, deleted_at, deleted_by,
owner_user_id, owner_org_id, confidentiality_level, version, is_locked,
locked_at, locked_by, retention_until, sort_final, metadata`

const insertRecordMetadataSQL = `
INSERT INTO record_metadata (
    subject_id, created_by, owner_user_id, owner_org_id,
    confidentiality_level, retention_until, sort_final, metadata
) VALUES (@subject_id, @created_by, @owner_user_id, @owner_org_id, @confidentiality_level, @retention_until, @sort_final, @metadata)
RETURNING ` + recordMetadataColumns + `;`

const getRecordMetadataSQL = `
SELECT ` + recordMetadataColumns + `
FROM record_metadata
WHERE subject_id = @subject_id;`

// getRecordMetadataForUpdateSQL locks the governance row for the duration of the
// transaction so a check-then-mutate (lock/deleted guard) is atomic against races.
const getRecordMetadataForUpdateSQL = `
SELECT ` + recordMetadataColumns + `
FROM record_metadata
WHERE subject_id = @subject_id
FOR UPDATE;`

// lockRecordMetadataSQL sets the immutable flag and bumps the version.
const lockRecordMetadataSQL = `
UPDATE record_metadata
SET is_locked = true, locked_at = now(), locked_by = @operator_id,
    updated_at = now(), updated_by = @operator_id, version = version + 1
WHERE subject_id = @subject_id
RETURNING ` + recordMetadataColumns + `;`

// softDeleteRecordMetadataSQL marks the governance record logically deleted.
const softDeleteRecordMetadataSQL = `
UPDATE record_metadata
SET deleted_at = now(), deleted_by = @operator_id, updated_at = now(), updated_by = @operator_id,
    version = version + 1
WHERE subject_id = @subject_id
RETURNING ` + recordMetadataColumns + `;`

// --- audit_event -------------------------------------------------------------

const auditEventColumns = `
id, subject_id, event_type, actor_user_id, occurred_at,
before_state, after_state, reason, correlation_id, request_id, metadata`

const insertAuditEventSQL = `
INSERT INTO audit_event (
    subject_id, event_type, actor_user_id, before_state, after_state,
    reason, correlation_id, request_id, metadata
) VALUES (@subject_id, @event_type, @actor_user_id, @before_state, @after_state, @reason, @correlation_id, @request_id, @metadata)
RETURNING ` + auditEventColumns + `;`

const listAuditEventsColumns = auditEventColumns + `,
COUNT(*) OVER() AS total_count`

const listAuditEventsSQL = `
SELECT ` + listAuditEventsColumns + `
FROM audit_event
WHERE subject_id = @subject_id
  AND (@event_type = '' OR event_type = @event_type)
  AND (@from::timestamptz IS NULL OR occurred_at >= @from)
  AND (@to::timestamptz IS NULL OR occurred_at <= @to)
ORDER BY occurred_at DESC
LIMIT @limit OFFSET @offset;`

// --- relationship_type -------------------------------------------------------

const relationshipTypeColumns = `
id, code, label, source_kind, target_kind, is_directed, inverse_label, description, is_active`

const getRelationshipTypeByCodeSQL = `
SELECT ` + relationshipTypeColumns + `
FROM relationship_type
WHERE code = @code;`

const getRelationshipTypesByIDsSQL = `
SELECT ` + relationshipTypeColumns + `
FROM relationship_type
WHERE id = ANY(@ids::uuid[]);`

const listRelationshipTypesSQL = `
SELECT ` + relationshipTypeColumns + `
FROM relationship_type
WHERE (NOT @only_active OR is_active = true)
  AND (@source_kind = '' OR source_kind = @source_kind)
  AND (@target_kind = '' OR target_kind = @target_kind)
ORDER BY code;`

// --- subject_relationship ----------------------------------------------------

const subjectRelationshipColumns = `
id, source_subject_id, target_subject_id, relationship_type_id, role_detail,
valid_from, valid_to, created_at, created_by, deleted_at`

const insertSubjectRelationshipSQL = `
INSERT INTO subject_relationship (
    source_subject_id, target_subject_id, relationship_type_id, role_detail, valid_from, created_by
) VALUES (@source_subject_id, @target_subject_id, @relationship_type_id, @role_detail, @valid_from, @created_by)
RETURNING ` + subjectRelationshipColumns + `;`

const softDeleteRelationshipSQL = `
UPDATE subject_relationship
SET deleted_at = now(), deleted_by = @operator_id
WHERE id = @id AND deleted_at IS NULL
RETURNING ` + subjectRelationshipColumns + `;`

// subjectRelationshipListColumns qualifies each column with the sr alias because
// listRelationshipsSQL joins relationship_type (which also has id/... columns);
// an unqualified projection would be ambiguous. Output column names are unchanged,
// so the relationshipListRow db tags still match.
const subjectRelationshipListColumns = `
sr.id, sr.source_subject_id, sr.target_subject_id, sr.relationship_type_id, sr.role_detail,
sr.valid_from, sr.valid_to, sr.created_at, sr.created_by, sr.deleted_at`

const listRelationshipsColumns = subjectRelationshipListColumns + `,
COUNT(*) OVER() AS total_count`

// listRelationshipsSQL lists active edges either outgoing from (@outgoing = true) or
// incoming to (@outgoing = false) the given subject, optionally filtered by type code.
const listRelationshipsSQL = `
SELECT ` + listRelationshipsColumns + `
FROM subject_relationship sr
JOIN relationship_type rt ON rt.id = sr.relationship_type_id
WHERE sr.deleted_at IS NULL
  AND ((@outgoing AND sr.source_subject_id = @subject_id) OR (NOT @outgoing AND sr.target_subject_id = @subject_id))
  AND (@relationship_type_code = '' OR rt.code = @relationship_type_code)
ORDER BY sr.created_at DESC
LIMIT @limit OFFSET @offset;`
