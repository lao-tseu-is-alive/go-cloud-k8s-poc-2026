package actor

// SQL fragments for the actor repository. Column projections are the single
// source of truth for pgx named scanning (columns map to `db` tags).
//
// Queries use pgx v5 named parameters (@name) bound through pgx.NamedArgs at the
// call sites, matching the document repository style.

const actorColumns = `
a.id, a.actor_kind, a.display_name, a.name_for_search, a.is_active, a.publication_code,
a.legal_name, a.organization_category_id, a.org_complement, a.is_ch_register, a.ch_register_ref,
a.created_at, a.created_by, a.updated_at`

const insertActorSQL = `
INSERT INTO actor AS a (
    id, actor_kind, display_name, name_for_search, is_active, publication_code,
    legal_name, organization_category_id, org_complement, is_ch_register, ch_register_ref, created_by
) VALUES (
    @id, @actor_kind, @display_name, @name_for_search, @is_active, @publication_code,
    @legal_name, @organization_category_id, @org_complement, @is_ch_register, @ch_register_ref, @created_by
)
RETURNING ` + actorColumns + `;`

const getActorSQL = `
SELECT ` + actorColumns + `
FROM actor a
WHERE a.id = @id;`

// updateActorSQL applies a partial update: each column is only replaced when its
// matching @set_* flag is true, otherwise the current value is kept via COALESCE
// on the flag. name_for_search is recomputed from the (possibly new) display_name.
const updateActorSQL = `
UPDATE actor a
SET display_name             = CASE WHEN @set_display_name THEN @display_name ELSE a.display_name END,
    name_for_search          = CASE WHEN @set_display_name THEN @name_for_search ELSE a.name_for_search END,
    is_active                = CASE WHEN @set_is_active THEN @is_active ELSE a.is_active END,
    publication_code         = CASE WHEN @set_publication_code THEN @publication_code ELSE a.publication_code END,
    legal_name               = CASE WHEN @set_legal_name THEN @legal_name ELSE a.legal_name END,
    organization_category_id = CASE WHEN @set_category THEN @organization_category_id ELSE a.organization_category_id END,
    org_complement           = CASE WHEN @set_org_complement THEN @org_complement ELSE a.org_complement END,
    is_ch_register           = CASE WHEN @set_is_ch_register THEN @is_ch_register ELSE a.is_ch_register END,
    ch_register_ref          = CASE WHEN @set_ch_register_ref THEN @ch_register_ref ELSE a.ch_register_ref END
WHERE a.id = @id
RETURNING ` + actorColumns + `;`

// --- actor_contact -----------------------------------------------------------

const contactColumns = `id, actor_id, contact_type, value, is_primary, label, created_at`

const insertContactSQL = `
INSERT INTO actor_contact (actor_id, contact_type, value, is_primary, label)
VALUES (@actor_id, @contact_type, @value, @is_primary, @label);`

const listContactsSQL = `
SELECT ` + contactColumns + `
FROM actor_contact
WHERE actor_id = @actor_id
ORDER BY contact_type, created_at;`

const deleteContactsSQL = `DELETE FROM actor_contact WHERE actor_id = @actor_id;`

// --- organization_category ---------------------------------------------------

const categoryColumns = `id, code, label, is_active`

const getCategoryByCodeSQL = `
SELECT ` + categoryColumns + `
FROM organization_category
WHERE code = @code;`

const getCategoryByIDSQL = `
SELECT ` + categoryColumns + `
FROM organization_category
WHERE id = @id;`

const listCategoriesSQL = `
SELECT ` + categoryColumns + `
FROM organization_category
WHERE (NOT @only_active OR is_active = true)
ORDER BY label;`

// --- search ------------------------------------------------------------------

const searchActorColumns = actorColumns + `,
COUNT(*) OVER() AS total_count`

// searchActorsSQL performs accent-insensitive name search over the generated
// search_vector plus kind / category / status filters. The query term is folded
// through immutable_unaccent (migration 0005) to match the accent-folded vector.
const searchActorsSQL = `
SELECT ` + searchActorColumns + `
FROM actor a
JOIN record_metadata rm ON rm.subject_id = a.id
WHERE (@query = '' OR a.search_vector @@ plainto_tsquery('simple', immutable_unaccent(@query)))
  AND (@actor_kind = 0 OR a.actor_kind = @actor_kind)
  AND (@category_code = '' OR a.organization_category_id = (SELECT id FROM organization_category WHERE code = @category_code))
  AND (NOT @only_active OR a.is_active = true)
  AND (@include_deleted OR rm.deleted_at IS NULL)
ORDER BY a.display_name
LIMIT @limit OFFSET @offset;`
