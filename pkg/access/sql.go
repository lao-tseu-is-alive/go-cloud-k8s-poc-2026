package access

// --- grants -------------------------------------------------------------------------------

// grantColumns projects a grant (alias g) with its grantee's display name: the
// user's name (else e-mail, else id) or the group or unit subject label.
const grantColumns = `
g.id, g.subject_id, g.grantee_kind, g.grantee_user_id, g.grantee_subject_id, g.level,
g.granted_at, g.granted_by, g.grant_reason, g.revoked_at, g.revoked_by, g.revoke_reason,
COALESCE(CASE WHEN g.grantee_kind = 'USER'
              THEN COALESCE(NULLIF(u.display_name, ''), NULLIF(u.email, ''), g.grantee_user_id)
              ELSE gs.display_label END, '') AS grantee_label`

const grantFrom = `
FROM access_grant g
LEFT JOIN app_user u ON u.user_id = g.grantee_user_id
LEFT JOIN subject_ref gs ON gs.id = g.grantee_subject_id`

const listGrantsSQL = `
SELECT ` + grantColumns + grantFrom + `
WHERE g.subject_id = @subject_id AND (@include_revoked OR g.revoked_at IS NULL)
ORDER BY (g.revoked_at IS NULL) DESC, g.level DESC, g.granted_at DESC, g.id;`

const getGrantSQL = `
SELECT ` + grantColumns + grantFrom + `
WHERE g.id = @id;`

// currentGrantSQL finds the current grant of one grantee (user or subject).
const currentGrantSQL = `
SELECT ` + grantColumns + grantFrom + `
WHERE g.subject_id = @subject_id AND g.revoked_at IS NULL AND g.grantee_kind = @grantee_kind
  AND (g.grantee_user_id = @grantee_user_id OR g.grantee_subject_id = @grantee_subject_id);`

// lockSubjectGrantsSQL serializes the grant changes of one subject, so the
// "last FULL_CONTROL" check cannot race.
const lockSubjectGrantsSQL = `SELECT pg_advisory_xact_lock(hashtextextended('goeland:access:' || @subject_id::text, 0));`

const insertGrantSQL = `
WITH g AS (
    INSERT INTO access_grant (subject_id, grantee_kind, grantee_user_id, grantee_subject_id, level, granted_by, grant_reason)
    VALUES (@subject_id, @grantee_kind, @grantee_user_id, @grantee_subject_id, @level, @granted_by, @grant_reason)
    RETURNING *
)
SELECT ` + grantColumns + `
FROM g
LEFT JOIN app_user u ON u.user_id = g.grantee_user_id
LEFT JOIN subject_ref gs ON gs.id = g.grantee_subject_id;`

const revokeGrantSQL = `
UPDATE access_grant
SET revoked_at = now(), revoked_by = @revoked_by, revoke_reason = @revoke_reason
WHERE id = @id AND revoked_at IS NULL;`

const countFullControlSQL = `
SELECT count(*) FROM access_grant
WHERE subject_id = @subject_id AND revoked_at IS NULL AND level = 4;`

const userKnownSQL = `SELECT EXISTS (SELECT 1 FROM app_user WHERE user_id = @user_id);`

const liveGroupSQL = `SELECT archived_at IS NULL FROM security_group WHERE id = @id;`

// --- groups -------------------------------------------------------------------------------

const groupColumns = `
sg.id, sg.name, sg.description, sg.archived_at, sg.archived_by,
(SELECT count(*) FROM subject_relationship r
   JOIN relationship_type rt ON rt.id = r.relationship_type_id AND rt.code = 'USER_MEMBER_OF_GROUP'
  WHERE r.target_subject_id = sg.id AND r.deleted_at IS NULL AND r.valid_to IS NULL)::int AS member_count`

const listGroupsSQL = `
SELECT ` + groupColumns + `
FROM security_group sg
WHERE (@include_archived OR sg.archived_at IS NULL)
  AND (@query = '' OR immutable_unaccent(lower(sg.name)) LIKE '%' || immutable_unaccent(lower(@query)) || '%')
ORDER BY lower(sg.name), sg.id;`

const getGroupSQL = `
SELECT ` + groupColumns + `
FROM security_group sg
WHERE sg.id = @id;`

const lockGroupSQL = `
SELECT ` + groupColumns + `
FROM security_group sg
WHERE sg.id = @id
FOR UPDATE OF sg;`

const insertGroupSQL = `
INSERT INTO security_group AS sg (id, name, description)
VALUES (@id, @name, @description)
RETURNING ` + groupColumns + `;`

const updateGroupSQL = `
UPDATE security_group AS sg
SET name = @name, description = @description
WHERE sg.id = @id
RETURNING ` + groupColumns + `;`

const archiveGroupSQL = `
UPDATE security_group AS sg
SET archived_at = now(), archived_by = @archived_by
WHERE sg.id = @id
RETURNING ` + groupColumns + `;`

// memberColumns projects a current membership (relationship r) with its user.
const memberFrom = `
SELECT u.user_id, u.subject_id AS user_subject_id, u.display_name, u.email,
       r.id AS relationship_id, COALESCE(r.valid_from, r.created_at) AS since, r.created_by AS added_by
FROM subject_relationship r
JOIN relationship_type rt ON rt.id = r.relationship_type_id AND rt.code = 'USER_MEMBER_OF_GROUP'
JOIN app_user u ON u.subject_id = r.source_subject_id
WHERE r.target_subject_id = @group_id AND r.deleted_at IS NULL AND r.valid_to IS NULL`

const listMembersSQL = memberFrom + `
ORDER BY lower(u.display_name), u.user_id;`

const getMemberSQL = memberFrom + `
  AND u.user_id = @user_id;`

const userSubjectSQL = `SELECT subject_id FROM app_user WHERE user_id = @user_id;`
