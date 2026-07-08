package document

// SQL fragments for the document repository. Column projections are the single
// source of truth for pgx named scanning (columns map to `db` tags).
//
// Queries use pgx v5 named parameters (@name) bound through pgx.NamedArgs at the
// call sites. Named binding keeps the argument list self-documenting and lets the
// SQL and the Go call site be read side by side without counting positions.

const documentColumns = `
d.id, d.document_type_id, d.title, d.description, d.official_date, d.storage_ref,
d.external_system, d.external_id, d.external_url, d.mime_type, d.file_size_bytes,
d.sha256, d.sha256_verified_at, d.version, d.previous_version_id, d.is_final,
d.is_record, d.language, d.page_count, d.status, d.metadata, d.created_at, d.created_by, d.updated_at`

const insertDocumentSQL = `
INSERT INTO document AS d (
    id, document_type_id, title, description, official_date, storage_ref,
    external_system, external_id, external_url, mime_type, file_size_bytes, sha256,
    version, previous_version_id, is_final, is_record, language, page_count, status, metadata, created_by
) VALUES (
    @id, @document_type_id, @title, @description, @official_date, @storage_ref,
    @external_system, @external_id, @external_url, @mime_type, @file_size_bytes, @sha256,
    @version, @previous_version_id, @is_final, @is_record, @language, @page_count, @status, @metadata, @created_by
)
RETURNING ` + documentColumns + `;`

const getDocumentSQL = `
SELECT ` + documentColumns + `
FROM document d
WHERE d.id = @id;`

const updateDocumentMetadataSQL = `
UPDATE document d
SET title = @title, description = @description, official_date = @official_date, language = @language, metadata = @metadata
WHERE d.id = @id
RETURNING ` + documentColumns + `;`

const finalizeDocumentSQL = `
UPDATE document d
SET is_final = true, status = 2
WHERE d.id = @id
RETURNING ` + documentColumns + `;`

// --- document_type -----------------------------------------------------------

const documentTypeColumns = `id, code, label, description, category, is_active`

const getDocumentTypeByCodeSQL = `
SELECT ` + documentTypeColumns + `
FROM document_type
WHERE code = @code;`

const getDocumentTypesByIDsSQL = `
SELECT ` + documentTypeColumns + `
FROM document_type
WHERE id = ANY(@ids::uuid[]);`

const listDocumentTypesSQL = `
SELECT ` + documentTypeColumns + `
FROM document_type
WHERE (NOT @only_active OR is_active = true)
ORDER BY code;`

// --- search ------------------------------------------------------------------

const searchDocumentColumns = documentColumns + `,
COUNT(*) OVER() AS total_count`

// searchDocumentsSQL performs full-text search over the generated tsvector plus
// governance and relationship filters.
// The query term is folded through immutable_unaccent() (migration 0005) so it
// matches the equally accent-folded search_vector: "chateau" finds "château".
const searchDocumentsSQL = `
SELECT ` + searchDocumentColumns + `
FROM document d
JOIN record_metadata rm ON rm.subject_id = d.id
WHERE (@query = '' OR d.search_vector @@ plainto_tsquery('simple', immutable_unaccent(@query)))
  AND (@document_type_code = '' OR d.document_type_id = (SELECT id FROM document_type WHERE code = @document_type_code))
  AND rm.confidentiality_level <= @confidentiality_max
  AND (NOT @only_records OR d.is_record)
  AND (NOT @only_final OR d.is_final)
  AND (@include_deleted OR rm.deleted_at IS NULL)
  AND (@case_id::uuid IS NULL OR EXISTS (
        SELECT 1 FROM subject_relationship sr
        JOIN relationship_type rt ON rt.id = sr.relationship_type_id
        WHERE sr.deleted_at IS NULL
          AND sr.target_subject_id = d.id
          AND sr.source_subject_id = @case_id
          AND rt.code = 'CASE_HAS_DOCUMENT'))
  AND (@thing_id::uuid IS NULL OR EXISTS (
        SELECT 1 FROM subject_relationship sr
        WHERE sr.deleted_at IS NULL
          AND sr.source_subject_id = d.id
          AND sr.target_subject_id = @thing_id))
ORDER BY d.created_at DESC
LIMIT @limit OFFSET @offset;`
