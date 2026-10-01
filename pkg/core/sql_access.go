package core

// --- grants (GLD-048) -------------------------------------------------------------------

// effectiveAccessSQL computes, for @user_id on @subject_id, the level each layer
// gives (NULL when it gives nothing) plus the subject's kind and
// confidentiality; EffectiveAccessTx applies the precedence. Units walk from
// the user's live units up to the root (a unit grant covers its sub-units); the
// nearest unit carrying a grant wins, the highest grant on ties.
const effectiveAccessSQL = `
WITH RECURSIVE
me AS (
    SELECT u.subject_id FROM app_user u WHERE u.user_id = @user_id
),
up AS (
    SELECT r.target_subject_id AS unit_id, 0 AS depth
    FROM subject_relationship r
    JOIN relationship_type rt ON rt.id = r.relationship_type_id AND rt.code = 'USER_MEMBER_OF_ORG_UNIT'
    JOIN org_unit ou ON ou.id = r.target_subject_id AND ou.dissolved_at IS NULL
    WHERE r.source_subject_id = (SELECT subject_id FROM me) AND r.deleted_at IS NULL AND r.valid_to IS NULL
    UNION ALL
    SELECT ou.parent_id, up.depth + 1
    FROM up
    JOIN org_unit ou ON ou.id = up.unit_id
    WHERE ou.parent_id IS NOT NULL AND up.depth < 64
),
my_groups AS (
    SELECT r.target_subject_id AS group_id
    FROM subject_relationship r
    JOIN relationship_type rt ON rt.id = r.relationship_type_id AND rt.code = 'USER_MEMBER_OF_GROUP'
    JOIN security_group sg ON sg.id = r.target_subject_id AND sg.archived_at IS NULL
    WHERE r.source_subject_id = (SELECT subject_id FROM me) AND r.deleted_at IS NULL AND r.valid_to IS NULL
),
grants AS (
    SELECT g.grantee_kind, g.grantee_user_id, g.grantee_subject_id, g.level
    FROM access_grant g
    WHERE g.subject_id = @subject_id AND g.revoked_at IS NULL
)
SELECT sr.kind,
       COALESCE(rm.confidentiality_level, 0) AS confidentiality,
       (SELECT max(g.level) FROM grants g WHERE g.grantee_kind = 'USER' AND g.grantee_user_id = @user_id) AS personal,
       (SELECT max(g.level) FROM grants g JOIN my_groups m ON m.group_id = g.grantee_subject_id
         WHERE g.grantee_kind = 'GROUP') AS group_level,
       (SELECT g.level FROM grants g JOIN up ON up.unit_id = g.grantee_subject_id
         WHERE g.grantee_kind = 'ORG_UNIT' ORDER BY up.depth, g.level DESC LIMIT 1) AS unit_level,
       (SELECT max(CASE WHEN ar.code = 'ADMIN' THEN 4 WHEN ar.scope_kind = sr.kind THEN ar.scope_level END)
          FROM app_user_role ur JOIN app_role ar ON ar.code = ur.role_code AND ar.is_active
         WHERE ur.user_id = @user_id AND ur.revoked_at IS NULL) AS role_level
FROM subject_ref sr
LEFT JOIN record_metadata rm ON rm.subject_id = sr.id
WHERE sr.id = @subject_id;`

const insertUserGrantSQL = `
INSERT INTO access_grant (subject_id, grantee_kind, grantee_user_id, level, granted_by, grant_reason)
VALUES (@subject_id, 'USER', @user_id, @level, @granted_by, @reason);`

const insertSubjectGrantSQL = `
INSERT INTO access_grant (subject_id, grantee_kind, grantee_subject_id, level, granted_by, grant_reason)
VALUES (@subject_id, @grantee_kind, @grantee_subject_id, @level, @granted_by, @reason);`

const relationshipTypeCodeSQL = `SELECT code FROM relationship_type WHERE id = @id;`

// copyGrantsSQL copies the current grants of @from to @to, skipping the
// grantees that already have a current grant on @to.
const copyGrantsSQL = `
INSERT INTO access_grant (subject_id, grantee_kind, grantee_user_id, grantee_subject_id, level, granted_by, grant_reason)
SELECT @to, g.grantee_kind, g.grantee_user_id, g.grantee_subject_id, g.level, @operator_id, 'copied from the case it was deposited in'
FROM access_grant g
WHERE g.subject_id = @from AND g.revoked_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM access_grant t
      WHERE t.subject_id = @to AND t.revoked_at IS NULL AND t.grantee_kind = g.grantee_kind
        AND (t.grantee_user_id = g.grantee_user_id OR t.grantee_subject_id = g.grantee_subject_id));`

// isUnitMemberSQL reports whether @user_id is a current direct member of @unit_id.
const isUnitMemberSQL = `
SELECT EXISTS (
    SELECT 1
    FROM subject_relationship r
    JOIN relationship_type rt ON rt.id = r.relationship_type_id AND rt.code = 'USER_MEMBER_OF_ORG_UNIT'
    JOIN app_user me ON me.subject_id = r.source_subject_id
    WHERE me.user_id = @user_id AND r.target_subject_id = @unit_id
      AND r.deleted_at IS NULL AND r.valid_to IS NULL);`

// viewerPrincipalsSQL lists the subjects a user's grants may come through: its
// live groups, and its live units with all their ancestors.
const viewerPrincipalsSQL = `
WITH RECURSIVE
me AS (SELECT u.subject_id FROM app_user u WHERE u.user_id = @user_id),
up AS (
    SELECT r.target_subject_id AS unit_id, 0 AS depth
    FROM subject_relationship r
    JOIN relationship_type rt ON rt.id = r.relationship_type_id AND rt.code = 'USER_MEMBER_OF_ORG_UNIT'
    JOIN org_unit ou ON ou.id = r.target_subject_id AND ou.dissolved_at IS NULL
    WHERE r.source_subject_id = (SELECT subject_id FROM me) AND r.deleted_at IS NULL AND r.valid_to IS NULL
    UNION ALL
    SELECT ou.parent_id, up.depth + 1
    FROM up JOIN org_unit ou ON ou.id = up.unit_id
    WHERE ou.parent_id IS NOT NULL AND up.depth < 64
)
SELECT unit_id FROM up
UNION
SELECT r.target_subject_id
FROM subject_relationship r
JOIN relationship_type rt ON rt.id = r.relationship_type_id AND rt.code = 'USER_MEMBER_OF_GROUP'
JOIN security_group sg ON sg.id = r.target_subject_id AND sg.archived_at IS NULL
WHERE r.source_subject_id = (SELECT subject_id FROM me) AND r.deleted_at IS NULL AND r.valid_to IS NULL;`
