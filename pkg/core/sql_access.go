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

// --- default grants (GLD-050) ---------------------------------------------------------

// upsertDefaultUserGrantSQL gives a template level to a user, raising an
// existing current grant (the creator's) only when the template is higher.
const upsertDefaultUserGrantSQL = `
INSERT INTO access_grant AS g (subject_id, grantee_kind, grantee_user_id, level, granted_by, grant_reason)
VALUES (@subject_id, 'USER', @user_id, @level, @granted_by, @reason)
ON CONFLICT (subject_id, grantee_user_id) WHERE revoked_at IS NULL AND grantee_kind = 'USER'
DO UPDATE SET level = EXCLUDED.level, grant_reason = EXCLUDED.grant_reason
WHERE g.level < EXCLUDED.level;`

// upsertDefaultSubjectGrantSQL gives a template level to a live group or unit,
// raising an existing current grant (the owning unit's) only when higher.
const upsertDefaultSubjectGrantSQL = `
INSERT INTO access_grant AS g (subject_id, grantee_kind, grantee_subject_id, level, granted_by, grant_reason)
SELECT @subject_id, @grantee_kind, @grantee_subject_id::uuid, @level, @granted_by, @reason
WHERE NOT EXISTS (SELECT 1 FROM security_group sg WHERE sg.id = @grantee_subject_id::uuid AND sg.archived_at IS NOT NULL)
  AND NOT EXISTS (SELECT 1 FROM org_unit ou WHERE ou.id = @grantee_subject_id::uuid AND ou.dissolved_at IS NOT NULL)
ON CONFLICT (subject_id, grantee_subject_id) WHERE revoked_at IS NULL AND grantee_kind <> 'USER'
DO UPDATE SET level = EXCLUDED.level, grant_reason = EXCLUDED.grant_reason
WHERE g.level < EXCLUDED.level;`

// creatorUnitsSQL lists the live units a user is a direct member of.
const creatorUnitsSQL = `
SELECT r.target_subject_id
FROM app_user u
JOIN subject_relationship r ON r.source_subject_id = u.subject_id AND r.deleted_at IS NULL AND r.valid_to IS NULL
JOIN relationship_type rt ON rt.id = r.relationship_type_id AND rt.code = 'USER_MEMBER_OF_ORG_UNIT'
JOIN org_unit ou ON ou.id = r.target_subject_id AND ou.dissolved_at IS NULL
WHERE u.user_id = @user_id
ORDER BY r.target_subject_id;`

const knownUserSQL = `SELECT EXISTS (SELECT 1 FROM app_user WHERE user_id = @user_id);`

const liveGroupGranteeSQL = `SELECT EXISTS (SELECT 1 FROM security_group WHERE id = @id AND archived_at IS NULL);`

const liveUnitGranteeSQL = `SELECT EXISTS (SELECT 1 FROM org_unit WHERE id = @id AND dissolved_at IS NULL);`

// --- sensitive read audit (GLD-033) ------------------------------------------------------

const insertAccessAuditSQL = `
INSERT INTO access_audit_event (user_id, subject_id, subject_kind, action, outcome, access_level,
                                access_source, confidentiality_level, document_version_id, request_id)
VALUES (@user_id, @subject_id, @subject_kind, @action, @outcome, @access_level,
        @access_source, @confidentiality_level, @document_version_id, @request_id);`

// accessAuditSortFields are the sortable columns of the sensitive read audit:
// the log grows with every confidential read, so only its indexed date sorts.
var accessAuditSortFields = map[string]SortField{
	"occurred_at": {Expr: "e.occurred_at"},
}

// defaultAccessAuditSort lists the latest accesses first.
var defaultAccessAuditSort = Sort{Field: "occurred_at", Desc: true}

// accessAuditColumns projects a log row with the subject's business reference
// and, only when the lister may read the subject, its label.
var accessAuditColumns = `
e.id, e.occurred_at, e.user_id, e.subject_id, e.subject_kind, e.action, e.outcome, e.access_level,
e.access_source, e.confidentiality_level, e.document_version_id, e.request_id,
(SELECT sr.business_ref FROM subject_ref sr WHERE sr.id = e.subject_id) AS business_ref,
CASE WHEN ` + ReadableSQL("e.subject_id", "") + `
     THEN (SELECT sr.display_label FROM subject_ref sr WHERE sr.id = e.subject_id) ELSE '' END AS subject_label`

// listAccessAuditSQL pages the log, filtered by subject, reader, action,
// outcome and period, in each sort (capped total, GLD-053).
var listAccessAuditSQL = SortedQueries(accessAuditSortFields, func(f SortField, desc bool) string {
	return CappedPageSQL(`
SELECT e.id AS id, `+f.Expr+` AS sort_key
FROM access_audit_event e
WHERE (@subject_id::uuid IS NULL OR e.subject_id = @subject_id::uuid)
  AND (@user_id = '' OR e.user_id = @user_id)
  AND (@action = '' OR e.action = @action)
  AND (@outcome = '' OR e.outcome = @outcome)
  AND (@from::timestamptz IS NULL OR e.occurred_at >= @from::timestamptz)
  AND (@to::timestamptz IS NULL OR e.occurred_at <= @to::timestamptz)`,
		accessAuditColumns, "access_audit_event", "e", f.Order(desc))
})
