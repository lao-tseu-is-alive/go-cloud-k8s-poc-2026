package core

// SQL fragments for the transversal core repository.
//
// Kept as raw SQL for full control. Column projections are the single source of
// truth for pgx named scanning (RowToStructByNameLax matches columns to `db` tags).

// --- subject_ref -------------------------------------------------------------

const subjectRefColumns = `id, kind, display_label, canonical_url, created_at`

const insertSubjectRefSQL = `
INSERT INTO subject_ref (kind, display_label, canonical_url)
VALUES ($1, $2, $3)
RETURNING ` + subjectRefColumns + `;`

const getSubjectRefSQL = `
SELECT ` + subjectRefColumns + `
FROM subject_ref
WHERE id = $1;`

const getSubjectRefsByIDsSQL = `
SELECT ` + subjectRefColumns + `
FROM subject_ref
WHERE id = ANY($1::uuid[]);`

// --- record_metadata ---------------------------------------------------------

const recordMetadataColumns = `
subject_id, created_at, created_by, updated_at, updated_by, deleted_at, deleted_by,
owner_user_id, owner_org_id, confidentiality_level, version, is_locked,
locked_at, locked_by, retention_until, sort_final, metadata`

const insertRecordMetadataSQL = `
INSERT INTO record_metadata (
    subject_id, created_by, owner_user_id, owner_org_id,
    confidentiality_level, retention_until, sort_final, metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING ` + recordMetadataColumns + `;`

const getRecordMetadataSQL = `
SELECT ` + recordMetadataColumns + `
FROM record_metadata
WHERE subject_id = $1;`

// lockRecordMetadataSQL sets the immutable flag and bumps the version.
const lockRecordMetadataSQL = `
UPDATE record_metadata
SET is_locked = true, locked_at = now(), locked_by = $2,
    updated_at = now(), updated_by = $2, version = version + 1
WHERE subject_id = $1
RETURNING ` + recordMetadataColumns + `;`

// softDeleteRecordMetadataSQL marks the governance record logically deleted.
const softDeleteRecordMetadataSQL = `
UPDATE record_metadata
SET deleted_at = now(), deleted_by = $2, updated_at = now(), updated_by = $2,
    version = version + 1
WHERE subject_id = $1
RETURNING ` + recordMetadataColumns + `;`

// --- audit_event -------------------------------------------------------------

const auditEventColumns = `
id, subject_id, event_type, actor_user_id, occurred_at,
before_state, after_state, reason, correlation_id, request_id, metadata`

const insertAuditEventSQL = `
INSERT INTO audit_event (
    subject_id, event_type, actor_user_id, before_state, after_state,
    reason, correlation_id, request_id, metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING ` + auditEventColumns + `;`

const listAuditEventsColumns = auditEventColumns + `,
COUNT(*) OVER() AS total_count`

const listAuditEventsSQL = `
SELECT ` + listAuditEventsColumns + `
FROM audit_event
WHERE subject_id = $1
  AND ($2 = '' OR event_type = $2)
  AND ($3::timestamptz IS NULL OR occurred_at >= $3)
  AND ($4::timestamptz IS NULL OR occurred_at <= $4)
ORDER BY occurred_at DESC
LIMIT $5 OFFSET $6;`

// --- relationship_type -------------------------------------------------------

const relationshipTypeColumns = `
id, code, label, source_kind, target_kind, is_directed, inverse_label, description, is_active`

const getRelationshipTypeByCodeSQL = `
SELECT ` + relationshipTypeColumns + `
FROM relationship_type
WHERE code = $1;`

const getRelationshipTypesByIDsSQL = `
SELECT ` + relationshipTypeColumns + `
FROM relationship_type
WHERE id = ANY($1::uuid[]);`

const listRelationshipTypesSQL = `
SELECT ` + relationshipTypeColumns + `
FROM relationship_type
WHERE (NOT $1 OR is_active = true)
  AND ($2 = '' OR source_kind = $2)
  AND ($3 = '' OR target_kind = $3)
ORDER BY code;`

// --- subject_relationship ----------------------------------------------------

const subjectRelationshipColumns = `
id, source_subject_id, target_subject_id, relationship_type_id, role_detail,
valid_from, valid_to, created_at, created_by, deleted_at`

const insertSubjectRelationshipSQL = `
INSERT INTO subject_relationship (
    source_subject_id, target_subject_id, relationship_type_id, role_detail, valid_from, created_by
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING ` + subjectRelationshipColumns + `;`

const softDeleteRelationshipSQL = `
UPDATE subject_relationship
SET deleted_at = now(), deleted_by = $2
WHERE id = $1 AND deleted_at IS NULL
RETURNING ` + subjectRelationshipColumns + `;`

const listRelationshipsColumns = subjectRelationshipColumns + `,
COUNT(*) OVER() AS total_count`

// listRelationshipsSQL lists active edges either outgoing from ($2 = true) or
// incoming to ($2 = false) the given subject, optionally filtered by type code.
const listRelationshipsSQL = `
SELECT ` + listRelationshipsColumns + `
FROM subject_relationship sr
JOIN relationship_type rt ON rt.id = sr.relationship_type_id
WHERE sr.deleted_at IS NULL
  AND (($2 AND sr.source_subject_id = $1) OR (NOT $2 AND sr.target_subject_id = $1))
  AND ($3 = '' OR rt.code = $3)
ORDER BY sr.created_at DESC
LIMIT $4 OFFSET $5;`
