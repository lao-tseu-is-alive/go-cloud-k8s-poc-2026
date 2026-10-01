package orgunit

import "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"

// SQL fragments for the org unit repository. Column projections are the single
// source of truth for pgx named scanning (columns map to `db` tags); they are
// alias-prefixed, so INSERT statements alias their target (AS u).

const unitColumns = `
u.id, u.org_unit_type_id, u.abbreviation, u.label, u.description, u.email, u.parent_id,
u.external_ref, u.dissolved_at, u.dissolved_by, u.dissolution_reason, u.created_at, u.created_by, u.updated_at`

// nodeColumns is the light projection of a unit (needs the type joined as t).
const nodeColumns = `
u.id, u.abbreviation, u.label, t.code AS type_code, u.parent_id, u.dissolved_at IS NOT NULL AS dissolved`

const insertUnitSQL = `
INSERT INTO org_unit AS u (id, org_unit_type_id, abbreviation, label, description, email, parent_id, external_ref, created_by)
VALUES (@id, @org_unit_type_id, @abbreviation, @label, @description, @email, @parent_id, @external_ref, @created_by)
RETURNING ` + unitColumns + `;`

const getUnitSQL = `
SELECT ` + unitColumns + `
FROM org_unit u
WHERE u.id = @id;`

// getUnitForUpdateSQL locks the unit row for a check-then-mutate.
const getUnitForUpdateSQL = `
SELECT ` + unitColumns + `
FROM org_unit u
WHERE u.id = @id
FOR UPDATE;`

const updateUnitSQL = `
UPDATE org_unit u
SET org_unit_type_id = @org_unit_type_id, abbreviation = @abbreviation, label = @label, description = @description,
    email = @email, parent_id = @parent_id
WHERE u.id = @id
RETURNING ` + unitColumns + `;`

const dissolveUnitSQL = `
UPDATE org_unit u
SET dissolved_at = now(), dissolved_by = @operator_id, dissolution_reason = @reason
WHERE u.id = @id
RETURNING ` + unitColumns + `;`

// listNodesSQL is the whole tree, ordered by type order then label.
var listNodesSQL = `
SELECT ` + nodeColumns + `
FROM org_unit u
JOIN org_unit_type t ON t.id = u.org_unit_type_id
WHERE (@include_dissolved OR u.dissolved_at IS NULL)
  AND ` + core.ReadableSQL("u.id", "") + `
ORDER BY t.sort_order, u.label, u.id;`

// ancestorsSQL walks up from a unit to the root; the recursion depth orders
// the path, returned from the root down.
const ancestorsSQL = `
WITH RECURSIVE path AS (
    SELECT parent_id AS id, 1 AS depth FROM org_unit WHERE id = @id AND parent_id IS NOT NULL
    UNION ALL
    SELECT o.parent_id, p.depth + 1 FROM org_unit o JOIN path p ON o.id = p.id WHERE o.parent_id IS NOT NULL
)
SELECT ` + nodeColumns + `
FROM path p
JOIN org_unit u ON u.id = p.id
JOIN org_unit_type t ON t.id = u.org_unit_type_id
ORDER BY p.depth DESC;`

const childrenSQL = `
SELECT ` + nodeColumns + `
FROM org_unit u
JOIN org_unit_type t ON t.id = u.org_unit_type_id
WHERE u.parent_id = @id
ORDER BY u.label, u.id;`

const countLiveChildrenSQL = `
SELECT count(*) FROM org_unit
WHERE parent_id = @id AND dissolved_at IS NULL;`

// isDescendantSQL tells whether @candidate lies in the subtree below @id.
const isDescendantSQL = `
WITH RECURSIVE below AS (
    SELECT id FROM org_unit WHERE parent_id = @id
    UNION
    SELECT o.id FROM org_unit o JOIN below b ON o.parent_id = b.id
)
SELECT EXISTS (SELECT 1 FROM below WHERE id = @candidate);`

// searchUnitsSQL matches the accent-folded search_vector, the exact
// abbreviation (case-insensitive) or the exact external reference, ordered by label.
var searchUnitsSQL = `
SELECT ` + unitColumns + `,
COUNT(*) OVER() AS total_count
FROM org_unit u
WHERE (@query = ''
       OR u.search_vector @@ plainto_tsquery('simple', immutable_unaccent(@query))
       OR lower(u.abbreviation) = lower(@query)
       OR (u.external_ref <> '' AND u.external_ref = @query))
  AND (@include_dissolved OR u.dissolved_at IS NULL)
  AND ` + core.ReadableSQL("u.id", "") + `
ORDER BY u.label, u.id
LIMIT @limit OFFSET @offset;`

// --- org_unit_type -----------------------------------------------------------------

const typeColumns = `id, code, label, description, sort_order, is_active`

const getTypeByCodeSQL = `
SELECT ` + typeColumns + `
FROM org_unit_type
WHERE code = @code;`

const getTypeByIDSQL = `
SELECT ` + typeColumns + `
FROM org_unit_type
WHERE id = @id;`

const getTypesByIDsSQL = `
SELECT ` + typeColumns + `
FROM org_unit_type
WHERE id = ANY(@ids::uuid[]);`

const listTypesSQL = `
SELECT ` + typeColumns + `
FROM org_unit_type
WHERE (NOT @only_active OR is_active = true)
ORDER BY sort_order, code;`

// --- org_unit_type administration (GLD-040) -----------------------------------------

const insertTypeSQL = `
INSERT INTO org_unit_type (code, label, description)
VALUES (@code, @label, @description)
RETURNING ` + typeColumns + `;`

const getTypeForUpdateSQL = `
SELECT ` + typeColumns + `
FROM org_unit_type
WHERE code = @code
FOR UPDATE;`

// updateTypeSQL replaces the fields given (NULL keeps the current value); the
// code is immutable.
const updateTypeSQL = `
UPDATE org_unit_type
SET label = coalesce(@label::text, label),
    description = coalesce(@description::text, description),
    is_active = coalesce(@is_active::boolean, is_active)
WHERE code = @code
RETURNING ` + typeColumns + `;`
