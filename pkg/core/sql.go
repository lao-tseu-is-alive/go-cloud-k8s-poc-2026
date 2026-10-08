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

const subjectRefColumns = `id, kind, display_label, canonical_url, business_ref, business_ref_namespace, created_at`

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

// assignBusinessRefSQL sets the business reference of a subject that has none
// yet; zero rows means the subject is unknown or already has a reference.
const assignBusinessRefSQL = `
UPDATE subject_ref
SET business_ref = @business_ref, business_ref_namespace = @business_ref_namespace
WHERE id = @id AND business_ref = ''
RETURNING ` + subjectRefColumns + `;`

// allocateBusinessRefSQL increments the (namespace, current year) counter and
// returns the new value; the upsert row-locks the counter until the transaction
// ends, serializing concurrent allocations. The year is computed in
// BusinessRefPeriodTimeZone by PostgreSQL (the scratch image has no tzdata).
const allocateBusinessRefSQL = `
INSERT INTO business_ref_counter AS c (namespace, period, last_value)
VALUES (@namespace, to_char(now() AT TIME ZONE @time_zone, 'YYYY'), 1)
ON CONFLICT (namespace, period) DO UPDATE SET last_value = c.last_value + 1
RETURNING period, last_value;`

// lookupSubjectsByBusinessRefSQL finds the subjects the viewer may read by exact
// business reference.
var lookupSubjectsByBusinessRefSQL = `
SELECT ` + subjectRefColumns + `
FROM subject_ref
WHERE business_ref = @business_ref
  AND (@business_ref_namespace = '' OR business_ref_namespace = @business_ref_namespace)
  AND (@kind = '' OR kind = @kind)
  AND ` + ReadableSQL("subject_ref.id", "") + `
ORDER BY created_at
LIMIT @limit;`

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

const getRecordMetadataByIDsSQL = `
SELECT ` + recordMetadataColumns + `
FROM record_metadata
WHERE subject_id = ANY(@ids::uuid[]);`

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
ORDER BY label, code;`

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

// getRelationshipForUpdateSQL locks one edge so EndRelationship checks its state
// and updates it atomically against a concurrent end or unlink.
const getRelationshipForUpdateSQL = `
SELECT ` + subjectRelationshipColumns + `
FROM subject_relationship
WHERE id = @id
FOR UPDATE;`

// endRelationshipSQL sets the business end of validity, defaulting to the
// database time; the validity-order CHECK (migration 0011) rejects an end before
// valid_from.
const endRelationshipSQL = `
UPDATE subject_relationship
SET valid_to = coalesce(@valid_to::timestamptz, now())
WHERE id = @id
RETURNING ` + subjectRelationshipColumns + `;`

// subjectRelationshipListColumns qualifies each column with the sr alias, the
// alias of the page join in listRelationshipsSQL. Output column names are
// unchanged, so the relationshipListRow db tags still match.
const subjectRelationshipListColumns = `
sr.id, sr.source_subject_id, sr.target_subject_id, sr.relationship_type_id, sr.role_detail,
sr.valid_from, sr.valid_to, sr.created_at, sr.created_by, sr.deleted_at`

// relationshipSortFields are the sortable columns of a relationship list
// (GLD-056). The labels are scalar subqueries rather than joins, so the scan
// starts from the subject's edges: with a join the planner sorted every
// subject_ref row to find the first labels (~5 s on the most linked subject).
var relationshipSortFields = map[string]SortField{
	"created_at":  {Expr: "sr.created_at"},
	"type":        {Expr: "(SELECT rt.label FROM relationship_type rt WHERE rt.id = sr.relationship_type_id)"},
	"source":      {Expr: "(SELECT s.display_label FROM subject_ref s WHERE s.id = sr.source_subject_id)"},
	"target":      {Expr: "(SELECT s.display_label FROM subject_ref s WHERE s.id = sr.target_subject_id)"},
	"role_detail": {Expr: "NULLIF(sr.role_detail, '')", Nullable: true},
	"valid_from":  {Expr: "sr.valid_from", Nullable: true},
}

// defaultRelationshipSort lists the latest edges first.
var defaultRelationshipSort = Sort{Field: "created_at", Desc: true}

// relationshipBranchSQL selects the edges of one direction (@flag): those
// whose subjectCol is the subject, optionally of one type, whose other end
// (otherCol) the viewer may read; each branch is ordered and cut at the count
// limit, so with the default order both read their index and stop early.
func relationshipBranchSQL(f SortField, order SortOrder, flag, subjectCol, otherCol string) string {
	return `(SELECT sr.id AS id, ` + f.Expr + ` AS sort_key
FROM subject_relationship sr
WHERE @` + flag + `::boolean AND sr.deleted_at IS NULL AND sr.` + subjectCol + ` = @subject_id
  AND (@relationship_type_code = '' OR sr.relationship_type_id = (SELECT rt.id FROM relationship_type rt WHERE rt.code = @relationship_type_code))
  AND ` + ReadableSQL("sr."+otherCol, "") + `
ORDER BY ` + sortOrderOf("", order) + `
LIMIT @count_limit)`
}

// listRelationshipsSQL lists non-unlinked edges (open and ended) outgoing from
// (@outgoing) and/or incoming to (@incoming) the given subject, optionally
// filtered by type code, whose other end the viewer may read, in each sort.
var listRelationshipsSQL = SortedQueries(relationshipSortFields, func(f SortField, desc bool) string {
	order := f.Order(desc)
	return CappedPageSQL(`
SELECT id, sort_key FROM (
`+relationshipBranchSQL(f, order, "outgoing", "source_subject_id", "target_subject_id")+`
UNION ALL
`+relationshipBranchSQL(f, order, "incoming", "target_subject_id", "source_subject_id")+`) u`,
		subjectRelationshipListColumns, "subject_relationship", "sr", order)
})

// --- app_user ------------------------------------------------------------------

// appUserColumns projects an app_user row (alias u) with its current roles:
// is_admin and roles are derived from app_user_role, never stored on the user.
const appUserColumns = `
u.user_id, u.subject_id, u.display_name, u.email, u.first_seen_at, u.last_seen_at,
EXISTS (SELECT 1 FROM app_user_role r WHERE r.user_id = u.user_id AND r.role_code = 'ADMIN' AND r.revoked_at IS NULL) AS is_admin,
ARRAY(SELECT r.role_code FROM app_user_role r WHERE r.user_id = u.user_id AND r.revoked_at IS NULL ORDER BY r.role_code) AS roles`

// lockAppUserSQL serializes the first recording of a user across concurrent
// requests (no row exists yet to lock) for the rest of the transaction.
const lockAppUserSQL = `SELECT pg_advisory_xact_lock(hashtextextended('goeland:app_user:' || @user_id, 0));`

const getAppUserForUpdateSQL = `
SELECT u.user_id, u.subject_id, u.display_name, u.email, u.first_seen_at, u.last_seen_at
FROM app_user u
WHERE u.user_id = @user_id
FOR UPDATE;`

const insertAppUserSQL = `
INSERT INTO app_user AS u (user_id, subject_id, display_name, email)
VALUES (@user_id, @subject_id, @display_name, @email)
RETURNING ` + appUserColumns + `;`

const updateAppUserSQL = `
UPDATE app_user AS u
SET display_name = @display_name, email = @email, last_seen_at = now()
WHERE u.user_id = @user_id
RETURNING ` + appUserColumns + `;`

const getAppUsersSQL = `
SELECT ` + appUserColumns + `
FROM app_user u
WHERE u.user_id = ANY(@user_ids::text[])
ORDER BY u.user_id;`

// searchAppUsersSQL matches the name (accent-insensitively) or the e-mail
// address by substring, ordered by name.
const searchAppUsersSQL = `
SELECT ` + appUserColumns + `
FROM app_user u
WHERE @query = ''
   OR immutable_unaccent(lower(u.display_name)) LIKE '%' || immutable_unaccent(lower(@query)) || '%'
   OR lower(u.email) LIKE '%' || lower(@query) || '%'
ORDER BY lower(u.display_name), u.user_id
LIMIT @limit;`

// --- application roles (GLD-047) ------------------------------------------------------

const appRoleColumns = `ar.code, ar.label, ar.description, ar.is_active`

const listAppRolesSQL = `
SELECT ` + appRoleColumns + `
FROM app_role ar
ORDER BY ar.code;`

const getAppRoleSQL = `
SELECT ` + appRoleColumns + `
FROM app_role ar
WHERE ar.code = @code;`

const userRoleColumns = `
ur.id, ur.user_id, ur.role_code, ur.granted_at, ur.granted_by, ur.grant_reason,
ur.revoked_at, ur.revoked_by, ur.revoke_reason`

const listUserRolesSQL = `
SELECT ` + userRoleColumns + `
FROM app_user_role ur
WHERE ur.user_id = @user_id AND (@include_revoked OR ur.revoked_at IS NULL)
ORDER BY ur.granted_at DESC, ur.id;`

const listRoleHoldersSQL = `
SELECT ` + appUserColumns + `
FROM app_user u
WHERE EXISTS (SELECT 1 FROM app_user_role r WHERE r.user_id = u.user_id AND r.role_code = @role_code AND r.revoked_at IS NULL)
ORDER BY lower(u.display_name), u.user_id;`

const activeRoleCodesSQL = `
SELECT ur.role_code
FROM app_user_role ur
WHERE ur.user_id = @user_id AND ur.revoked_at IS NULL
ORDER BY ur.role_code;`

// lockRoleSQL serializes the changes of one role, so two concurrent
// revocations cannot both pass the "last administrator" check.
const lockRoleSQL = `SELECT pg_advisory_xact_lock(hashtextextended('goeland:app_role:' || @role_code, 0));`

const insertUserRoleSQL = `
INSERT INTO app_user_role AS ur (user_id, role_code, granted_by, grant_reason)
VALUES (@user_id, @role_code, @granted_by, @grant_reason)
RETURNING ` + userRoleColumns + `;`

const revokeUserRoleSQL = `
UPDATE app_user_role AS ur
SET revoked_at = now(), revoked_by = @revoked_by, revoke_reason = @revoke_reason
WHERE ur.user_id = @user_id AND ur.role_code = @role_code AND ur.revoked_at IS NULL
RETURNING ` + userRoleColumns + `;`

const countRoleHoldersSQL = `
SELECT count(*) FROM app_user_role
WHERE role_code = @role_code AND revoked_at IS NULL;`

// --- reference_change ------------------------------------------------------------

const referenceChangeColumns = `
id, catalogue, code, event_type, actor_user_id, occurred_at, before_state, after_state, reason`

const insertReferenceChangeSQL = `
INSERT INTO reference_change (catalogue, code, event_type, actor_user_id, before_state, after_state, reason)
VALUES (@catalogue, @code, @event_type, @actor_user_id, @before_state, @after_state, @reason)
RETURNING ` + referenceChangeColumns + `;`

// referenceChangeSortFields are the sortable columns of the reference change
// log (GLD-056); the log is small (administrators' changes), so it sorts
// without dedicated indexes.
var referenceChangeSortFields = map[string]SortField{
	"occurred_at": {Expr: "occurred_at"},
	"catalogue":   {Expr: "catalogue"},
	"code":        {Expr: "code"},
	"event":       {Expr: "event_type"},
}

// defaultReferenceChangeSort lists the latest changes first.
var defaultReferenceChangeSort = Sort{Field: "occurred_at", Desc: true}

// listReferenceChangesSQL pages the log, optionally of one catalogue, in each sort.
var listReferenceChangesSQL = SortedQueries(referenceChangeSortFields, func(f SortField, desc bool) string {
	return `
SELECT ` + referenceChangeColumns + `, count(*) OVER () AS total_size
FROM reference_change
WHERE (@catalogue = '' OR catalogue = @catalogue)
ORDER BY ` + f.Expr + OrderDirection(f.Order(desc)) + `, occurred_at DESC, id
LIMIT @limit OFFSET @offset;`
})

const insertRelationshipTypeSQL = `
INSERT INTO relationship_type (code, label, source_kind, target_kind, is_directed, inverse_label, description)
VALUES (@code, @label, @source_kind, @target_kind, @is_directed, @inverse_label, @description)
RETURNING ` + relationshipTypeColumns + `;`

const getRelationshipTypeForUpdateSQL = `
SELECT ` + relationshipTypeColumns + `
FROM relationship_type
WHERE code = @code
FOR UPDATE;`

// updateRelationshipTypeSQL replaces the mutable fields given (NULL keeps the
// current value); code, kinds and direction are immutable.
const updateRelationshipTypeSQL = `
UPDATE relationship_type
SET label         = coalesce(@label::text, label),
    inverse_label = coalesce(@inverse_label::text, inverse_label),
    description   = coalesce(@description::text, description),
    is_active     = coalesce(@is_active::boolean, is_active)
WHERE code = @code
RETURNING ` + relationshipTypeColumns + `;`

// orgUnitDissolvedSQL tells whether an org unit is dissolved (no row: unknown unit).
const orgUnitDissolvedSQL = `
SELECT dissolved_at IS NOT NULL FROM org_unit
WHERE id = @id;`

// caseStatusSQL reads a case status (no row: the subject is not a case).
const caseStatusSQL = `
SELECT status FROM case_file
WHERE id = @id;`
