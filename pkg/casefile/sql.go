package casefile

import "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"

// SQL fragments for the case repository. Column projections are the single
// source of truth for pgx named scanning (columns map to `db` tags); they are
// alias-prefixed, so INSERT statements alias their target (AS c).

const caseColumns = `
c.id, c.case_type_id, c.title, c.description, c.status, c.opened_at, c.closed_at,
c.closed_by, c.closure_reason, c.metadata, c.created_at, c.created_by, c.updated_at`

const insertCaseSQL = `
INSERT INTO case_file AS c (id, case_type_id, title, description, metadata, created_by)
VALUES (@id, @case_type_id, @title, @description, @metadata, @created_by)
RETURNING ` + caseColumns + `;`

const getCaseSQL = `
SELECT ` + caseColumns + `
FROM case_file c
WHERE c.id = @id;`

// getCaseForUpdateSQL locks the case row for a check-then-mutate on its status.
const getCaseForUpdateSQL = `
SELECT ` + caseColumns + `
FROM case_file c
WHERE c.id = @id
FOR UPDATE;`

const updateCaseSQL = `
UPDATE case_file c
SET title = @title, description = @description, metadata = @metadata
WHERE c.id = @id
RETURNING ` + caseColumns + `;`

// transitionCaseSQL sets the status and keeps the closure stamps consistent
// with it (set when closing, cleared when leaving CLOSED).
const transitionCaseSQL = `
UPDATE case_file c
SET status = @status::smallint,
    closed_at = CASE WHEN @status::smallint = 4 THEN now() END,
    closed_by = CASE WHEN @status::smallint = 4 THEN @operator_id::text ELSE '' END,
    closure_reason = CASE WHEN @status::smallint = 4 THEN @reason::text ELSE '' END
WHERE c.id = @id
RETURNING ` + caseColumns + `;`

// --- case_type -------------------------------------------------------------------

const caseTypeColumns = `id, code, label, description, business_ref_namespace, is_active, default_confidentiality_level`

const getCaseTypeByCodeSQL = `
SELECT ` + caseTypeColumns + `
FROM case_type
WHERE code = @code;`

const getCaseTypesByIDsSQL = `
SELECT ` + caseTypeColumns + `
FROM case_type
WHERE id = ANY(@ids::uuid[]);`

const listCaseTypesSQL = `
SELECT ` + caseTypeColumns + `
FROM case_type
WHERE (NOT @only_active OR is_active = true)
ORDER BY code;`

// --- search ----------------------------------------------------------------------

// searchCasesSQL matches the accent-folded search_vector (immutable_unaccent,
// migration 0005) or the exact business reference, plus type/status/deletion filters,
// newest first, with a capped total (core.CappedPageSQL).
var searchCasesSQL = core.CappedPageSQL(`
SELECT c.id AS id, c.created_at AS sort_key
FROM case_file c
`+core.MetadataLateralSQL("c.id")+`
WHERE (@query = ''
       OR c.search_vector @@ plainto_tsquery('simple', immutable_unaccent(@query))
       OR EXISTS (SELECT 1 FROM subject_ref sr WHERE sr.id = c.id AND sr.business_ref = @query))
  AND (@case_type_code = '' OR c.case_type_id = (SELECT id FROM case_type WHERE code = @case_type_code))
  AND (@status::smallint = 0 OR c.status = @status::smallint)
  AND (@include_deleted OR rm.deleted_at IS NULL)
  AND `+core.ReadableSQL("c.id", "rm"), caseColumns, "case_file", "c", true)

// --- case_type administration (GLD-040) ---------------------------------------------

const insertCaseTypeSQL = `
INSERT INTO case_type (code, label, description, business_ref_namespace, default_confidentiality_level)
VALUES (@code, @label, @description, @business_ref_namespace, @default_confidentiality_level)
RETURNING ` + caseTypeColumns + `;`

const getCaseTypeForUpdateSQL = `
SELECT ` + caseTypeColumns + `
FROM case_type
WHERE code = @code
FOR UPDATE;`

// updateCaseTypeSQL replaces the fields given (NULL keeps the current value); the
// code is immutable.
const updateCaseTypeSQL = `
UPDATE case_type
SET label = coalesce(@label::text, label),
    description = coalesce(@description::text, description),
    business_ref_namespace = coalesce(@business_ref_namespace::text, business_ref_namespace),
    is_active = coalesce(@is_active::boolean, is_active),
    default_confidentiality_level = coalesce(@default_confidentiality_level::smallint, default_confidentiality_level)
WHERE code = @code
RETURNING ` + caseTypeColumns + `;`

// --- case type default grants (GLD-050) -----------------------------------------------

// listDefaultGrantsSQL returns the template lines of several case types with
// their grantee labels, in their order of definition.
const listDefaultGrantsSQL = `
SELECT g.case_type_id, g.grantee_kind, g.grantee_user_id, g.grantee_subject_id, g.level,
       coalesce(u.display_name, sr.display_label, '') AS grantee_label
FROM case_type_default_grant g
LEFT JOIN app_user u ON u.user_id = g.grantee_user_id
LEFT JOIN subject_ref sr ON sr.id = g.grantee_subject_id
WHERE g.case_type_id = ANY(@ids::uuid[])
ORDER BY g.created_at, g.id;`

const deleteDefaultGrantsSQL = `DELETE FROM case_type_default_grant WHERE case_type_id = @case_type_id;`

const insertDefaultGrantSQL = `
INSERT INTO case_type_default_grant (case_type_id, grantee_kind, grantee_user_id, grantee_subject_id, level, created_by)
VALUES (@case_type_id, @grantee_kind, @grantee_user_id, @grantee_subject_id, @level, @created_by);`
