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

// searchMetadataSQL joins the governance of subject s as rm, if any (a user
// has none).
const searchMetadataSQL = `LEFT JOIN LATERAL (SELECT r.deleted_at, r.confidentiality_level FROM record_metadata r WHERE r.subject_id = s.id LIMIT 1) rm ON true`

// searchVisibleSQL is "subject s is not deleted and the viewer may read it".
var searchVisibleSQL = `rm.deleted_at IS NULL AND ` + ReadableSQL("s.id", "rm")

// searchSubjectsSQL is the global search (GLD-064): per requested kind (in
// order, k.ord), the subjects whose reference equals @business_ref and the
// first @count_limit whose label or reference matches @text_query from the
// start of every word, read from idx_subject_ref_search through the kind
// token; deduplicated, counted, and the best @limit kept: exact reference,
// label starting with the first word, shortest label (ts_rank recomputed the
// vector of every candidate: +60 ms on a broad word), most recent.
var searchSubjectsSQL = `
SELECT k.ord, x.total_count,
       s.id, s.kind, s.display_label, s.canonical_url, s.business_ref, s.business_ref_namespace, s.created_at
FROM unnest(@kinds::text[]) WITH ORDINALITY AS k(kind, ord)
CROSS JOIN LATERAL (
  SELECT m.id, count(*) OVER () AS total_count,
         row_number() OVER (ORDER BY min(m.exact),
                                     (lower(immutable_unaccent(s.display_label)) LIKE @label_prefix) DESC,
                                     length(s.display_label),
                                     s.created_at DESC, m.id) AS rn
  FROM (
    (SELECT s.id, 0 AS exact
     FROM subject_ref s
     ` + searchMetadataSQL + `
     WHERE s.business_ref = @business_ref AND s.kind = k.kind AND ` + searchVisibleSQL + `
     LIMIT @limit)
    UNION ALL
    (SELECT s.id, 1 AS exact
     FROM subject_ref s
     ` + searchMetadataSQL + `
     WHERE ` + subjectSearchVectorSQL("s") + `
           @@ (to_tsquery('simple', immutable_unaccent(@text_query)) && ('''§' || lower(k.kind) || '''')::tsquery)
       AND ` + searchVisibleSQL + `
     LIMIT @count_limit)
  ) m
  JOIN subject_ref s ON s.id = m.id
  GROUP BY m.id, s.display_label, s.business_ref, s.kind, s.created_at
  ORDER BY rn
  LIMIT @limit
) x
JOIN subject_ref s ON s.id = x.id
ORDER BY k.ord, x.rn;`

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
id, code, label, source_kind, target_kind, is_directed, inverse_label, description, is_active,
role_label, role_label_plural, role_end, role_preposition`

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
  AND ` + otherEndFilterSQL("sr."+otherCol) + `
ORDER BY ` + sortOrderOf("", order) + `
LIMIT @count_limit)`
}

// otherEndFilterSQL keeps the edges whose other end (other) matches the
// optional filters of a relationship group (GLD-063): a case open or closed
// (@other_case_status), a case, document or thing type (@other_type_code), the
// year of a case's opening or a document's official date (@other_year_from,
// @other_year_to; 0 is unbounded), and a text in its label or business
// reference, accent- and case-insensitive (@other_query).
func otherEndFilterSQL(other string) string {
	return `(@other_query::text = '' OR EXISTS (
       SELECT 1 FROM subject_ref os WHERE os.id = ` + other + `
         AND strpos(immutable_unaccent(lower(os.display_label || ' ' || coalesce(os.business_ref, ''))),
                    immutable_unaccent(lower(@other_query::text))) > 0))
  AND (@other_case_status::text = '' OR EXISTS (
       SELECT 1 FROM case_file oc WHERE oc.id = ` + other + ` AND (oc.status = 4) = (@other_case_status::text = 'closed')))
  AND (@other_type_code::text = ''
       OR EXISTS (SELECT 1 FROM case_file oc JOIN case_type oct ON oct.id = oc.case_type_id
                  WHERE oc.id = ` + other + ` AND oct.code = @other_type_code::text)
       OR EXISTS (SELECT 1 FROM document od JOIN document_type odt ON odt.id = od.document_type_id
                  WHERE od.id = ` + other + ` AND odt.code = @other_type_code::text)
       OR EXISTS (SELECT 1 FROM thing ot JOIN thing_type ott ON ott.id = ot.thing_type_id
                  WHERE ot.id = ` + other + ` AND ott.code = @other_type_code::text))
  AND ((@other_year_from::int = 0 AND @other_year_to::int = 0) OR EXTRACT(YEAR FROM coalesce(
       (SELECT oc.opened_at FROM case_file oc WHERE oc.id = ` + other + `),
       (SELECT od.official_date::timestamptz FROM document od WHERE od.id = ` + other + `)))
       BETWEEN @other_year_from::int AND coalesce(NULLIF(@other_year_to::int, 0), 9999))`
}

// summarizeRelationshipsSQL counts a subject's non-unlinked edges per type,
// direction and case, document or thing type of the other end, keeping those whose
// other end the viewer may read (the same set the lists page through), with
// the other ends that are cases not closed (GLD-063).
var summarizeRelationshipsSQL = `
WITH e AS (
    SELECT sr.relationship_type_id AS type_id, true AS outgoing, sr.target_subject_id AS other_id
    FROM subject_relationship sr
    WHERE sr.deleted_at IS NULL AND sr.source_subject_id = @subject_id
    UNION ALL
    SELECT sr.relationship_type_id, false, sr.source_subject_id
    FROM subject_relationship sr
    WHERE sr.deleted_at IS NULL AND sr.target_subject_id = @subject_id
)
SELECT e.type_id, e.outgoing, s.kind AS other_kind,
       coalesce(ct.code, dt.code, tt.code, '') AS fam_code, coalesce(ct.label, dt.label, tt.label, '') AS fam_label,
       count(*)::int AS n,
       (count(*) FILTER (WHERE cf.status IS NOT NULL AND cf.status <> 4))::int AS open_n
FROM e
JOIN subject_ref s ON s.id = e.other_id
LEFT JOIN case_file cf ON cf.id = e.other_id
LEFT JOIN case_type ct ON ct.id = cf.case_type_id
LEFT JOIN document d ON d.id = e.other_id
LEFT JOIN document_type dt ON dt.id = d.document_type_id
LEFT JOIN thing t ON t.id = e.other_id
LEFT JOIN thing_type tt ON tt.id = t.thing_type_id
WHERE ` + ReadableSQL("e.other_id", "") + `
GROUP BY e.type_id, e.outgoing, s.kind, fam_code, fam_label
ORDER BY fam_label, fam_code;`

// relatedDetailsSQL describes subjects for a relationship list: their case,
// document or thing type, a case's state and the case opening or document
// official date (GLD-063).
const relatedDetailsSQL = `
SELECT s.id,
       coalesce(ct.code, dt.code, tt.code, '') AS type_code,
       coalesce(ct.label, dt.label, tt.label, '') AS type_label,
       CASE cf.status WHEN 1 THEN 'OPEN' WHEN 2 THEN 'IN_PROGRESS' WHEN 3 THEN 'SUSPENDED' WHEN 4 THEN 'CLOSED' ELSE '' END AS case_status,
       coalesce(to_char(cf.opened_at, 'YYYY-MM-DD'), to_char(d.official_date, 'YYYY-MM-DD'), '') AS date
FROM subject_ref s
LEFT JOIN case_file cf ON cf.id = s.id
LEFT JOIN case_type ct ON ct.id = cf.case_type_id
LEFT JOIN document d ON d.id = s.id
LEFT JOIN document_type dt ON dt.id = d.document_type_id
LEFT JOIN thing t ON t.id = s.id
LEFT JOIN thing_type tt ON tt.id = t.thing_type_id
WHERE s.id = ANY(@ids::uuid[]);`

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
INSERT INTO relationship_type (code, label, source_kind, target_kind, is_directed, inverse_label, description,
    role_label, role_label_plural, role_end, role_preposition)
VALUES (@code, @label, @source_kind, @target_kind, @is_directed, @inverse_label, @description,
    @role_label, @role_label_plural, @role_end, @role_preposition)
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
    is_active     = coalesce(@is_active::boolean, is_active),
    role_label        = coalesce(@role_label::text, role_label),
    role_label_plural = coalesce(@role_label_plural::text, role_label_plural),
    role_end          = coalesce(@role_end::text, role_end),
    role_preposition  = coalesce(@role_preposition::text, role_preposition)
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

// --- outbox_event (GLD-028) ----------------------------------------------------

const outboxEventColumns = `
o.id, o.subject_kind, o.subject_id, o.event_type, o.audit_event_id, o.payload, o.occurred_at,
o.correlation_id, o.request_id, o.attempts, o.next_attempt_at, o.published_at, o.failed_at,
o.last_error, o.failed_consumers`

// insertOutboxEventSQL announces an audit event to the outbox consumers and
// wakes the workers; the notification is sent when the transaction commits,
// and PostgreSQL folds the identical ones of a transaction into one.
const insertOutboxEventSQL = `
WITH o AS (
    INSERT INTO outbox_event (subject_kind, subject_id, event_type, audit_event_id, payload, occurred_at, correlation_id, request_id)
    VALUES (coalesce((SELECT kind FROM subject_ref WHERE id = @subject_id), ''), @subject_id, @event_type, @audit_event_id,
            @payload, @occurred_at, @correlation_id, @request_id)
    RETURNING id)
SELECT pg_notify('` + OutboxChannel + `', '') FROM o;`

// claimOutboxEventsSQL takes a batch of events due for delivery, oldest first,
// including those whose worker went silent for longer than @lease; SKIP LOCKED
// lets the workers of several replicas share the queue.
const claimOutboxEventsSQL = `
UPDATE outbox_event AS o
SET claimed_at = now(), attempts = o.attempts + 1
WHERE o.id IN (
    SELECT q.id FROM outbox_event q
    WHERE q.published_at IS NULL AND q.failed_at IS NULL
      AND q.next_attempt_at <= now()
      AND (q.claimed_at IS NULL OR q.claimed_at < now() - @lease::interval)
    ORDER BY q.occurred_at, q.id
    LIMIT @batch
    FOR UPDATE SKIP LOCKED)
RETURNING ` + outboxEventColumns + `;`

// publishOutboxEventsSQL records the events every consumer accepted.
const publishOutboxEventsSQL = `
UPDATE outbox_event
SET published_at = now(), claimed_at = NULL, last_error = '', failed_consumers = '{}'
WHERE id = ANY(@ids) AND published_at IS NULL AND failed_at IS NULL;`

// retryOutboxEventSQL records a failed delivery: tried again after @delay, or
// given up once @max_attempts are spent.
const retryOutboxEventSQL = `
UPDATE outbox_event
SET claimed_at = NULL, last_error = @last_error, failed_consumers = @failed_consumers,
    next_attempt_at = now() + @delay::interval,
    failed_at = CASE WHEN attempts >= @max_attempts THEN now() END
WHERE id = @id AND published_at IS NULL AND failed_at IS NULL;`

// failExhaustedOutboxSQL gives up on the events whose deliveries kept dying
// mid-way (their worker stopped before recording the outcome).
const failExhaustedOutboxSQL = `
UPDATE outbox_event
SET failed_at = now(), claimed_at = NULL, last_error = 'the delivery was interrupted too many times'
WHERE published_at IS NULL AND failed_at IS NULL AND attempts >= @max_attempts
  AND claimed_at < now() - @lease::interval;`

// purgeOutboxEventsSQL deletes a batch of events delivered before the retention.
const purgeOutboxEventsSQL = `
DELETE FROM outbox_event
WHERE id IN (
    SELECT id FROM outbox_event
    WHERE published_at < now() - @retention::interval
    LIMIT @batch);`

// outboxCountsSQL counts the events by state.
const outboxCountsSQL = `
SELECT
    count(*) FILTER (WHERE published_at IS NULL AND failed_at IS NULL) AS pending,
    count(*) FILTER (WHERE failed_at IS NOT NULL) AS failed,
    count(*) FILTER (WHERE published_at IS NOT NULL) AS published,
    min(occurred_at) FILTER (WHERE published_at IS NULL AND failed_at IS NULL) AS oldest_pending_at
FROM outbox_event;`

// outboxFailedByConsumerSQL counts the given-up events per consumer that refused them.
const outboxFailedByConsumerSQL = `
SELECT c AS name, count(*) AS failed
FROM outbox_event, unnest(failed_consumers) AS c
WHERE failed_at IS NOT NULL
GROUP BY c;`

// recentFailedOutboxSQL lists the latest given-up events.
const recentFailedOutboxSQL = `
SELECT ` + outboxEventColumns + `
FROM outbox_event o
WHERE o.failed_at IS NOT NULL
ORDER BY o.failed_at DESC, o.id DESC
LIMIT @limit;`

// replayOutboxEventsSQL puts failed events back in the queue: those listed in
// @ids, or every failed one when @all; it wakes the workers.
const replayOutboxEventsSQL = `
WITH r AS (
    UPDATE outbox_event
    SET failed_at = NULL, attempts = 0, next_attempt_at = now(), claimed_at = NULL,
        last_error = '', failed_consumers = '{}'
    WHERE failed_at IS NOT NULL AND (@all OR id = ANY(@ids))
    RETURNING id)
SELECT count(*) AS replayed, pg_notify('` + OutboxChannel + `', '') IS NULL AS notified FROM r;`
