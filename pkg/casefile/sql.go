package casefile

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

const caseTypeColumns = `id, code, label, description, business_ref_namespace, is_active`

const getCaseTypeByCodeSQL = `
SELECT ` + caseTypeColumns + `
FROM case_type
WHERE code = @code;`

const getCaseTypeByIDSQL = `
SELECT ` + caseTypeColumns + `
FROM case_type
WHERE id = @id;`

const listCaseTypesSQL = `
SELECT ` + caseTypeColumns + `
FROM case_type
WHERE (NOT @only_active OR is_active = true)
ORDER BY code;`

// --- search ----------------------------------------------------------------------

// searchCasesSQL matches the accent-folded search_vector (immutable_unaccent,
// migration 0005) or the exact business reference, plus type/status/deletion filters.
const searchCasesSQL = `
SELECT ` + caseColumns + `,
COUNT(*) OVER() AS total_count
FROM case_file c
JOIN subject_ref sr ON sr.id = c.id
JOIN record_metadata rm ON rm.subject_id = c.id
WHERE (@query = ''
       OR c.search_vector @@ plainto_tsquery('simple', immutable_unaccent(@query))
       OR sr.business_ref = @query)
  AND (@case_type_code = '' OR c.case_type_id = (SELECT id FROM case_type WHERE code = @case_type_code))
  AND (@status::smallint = 0 OR c.status = @status::smallint)
  AND (@include_deleted OR rm.deleted_at IS NULL)
ORDER BY c.created_at DESC
LIMIT @limit OFFSET @offset;`
