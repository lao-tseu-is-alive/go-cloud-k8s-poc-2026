package document

// SQL fragments for the document repository. Column projections are the single
// source of truth for pgx named scanning (columns map to `db` tags).

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
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11, $12,
    $13, $14, $15, $16, $17, $18, $19, $20, $21
)
RETURNING ` + documentColumns + `;`

const getDocumentSQL = `
SELECT ` + documentColumns + `
FROM document d
WHERE d.id = $1;`

const updateDocumentMetadataSQL = `
UPDATE document d
SET title = $2, description = $3, official_date = $4, language = $5, metadata = $6
WHERE d.id = $1
RETURNING ` + documentColumns + `;`

const finalizeDocumentSQL = `
UPDATE document d
SET is_final = true, status = 2
WHERE d.id = $1
RETURNING ` + documentColumns + `;`

const touchDocumentVerifiedAtSQL = `
UPDATE document d
SET sha256_verified_at = now()
WHERE d.id = $1
RETURNING ` + documentColumns + `;`

// --- document_type -----------------------------------------------------------

const documentTypeColumns = `id, code, label, description, category, is_active`

const getDocumentTypeByCodeSQL = `
SELECT ` + documentTypeColumns + `
FROM document_type
WHERE code = $1;`

const getDocumentTypesByIDsSQL = `
SELECT ` + documentTypeColumns + `
FROM document_type
WHERE id = ANY($1::uuid[]);`

const listDocumentTypesSQL = `
SELECT ` + documentTypeColumns + `
FROM document_type
WHERE (NOT $1 OR is_active = true)
ORDER BY code;`

// --- search ------------------------------------------------------------------

const searchDocumentColumns = documentColumns + `,
COUNT(*) OVER() AS total_count`

// searchDocumentsSQL performs full-text search over the generated tsvector plus
// governance and relationship filters.
const searchDocumentsSQL = `
SELECT ` + searchDocumentColumns + `
FROM document d
JOIN record_metadata rm ON rm.subject_id = d.id
WHERE ($1 = '' OR d.search_vector @@ plainto_tsquery('simple', $1))
  AND ($2 = '' OR d.document_type_id = (SELECT id FROM document_type WHERE code = $2))
  AND rm.confidentiality_level <= $3
  AND (NOT $4 OR d.is_record)
  AND (NOT $5 OR d.is_final)
  AND ($6 OR rm.deleted_at IS NULL)
  AND ($7::uuid IS NULL OR EXISTS (
        SELECT 1 FROM subject_relationship sr
        JOIN relationship_type rt ON rt.id = sr.relationship_type_id
        WHERE sr.deleted_at IS NULL
          AND sr.target_subject_id = d.id
          AND sr.source_subject_id = $7
          AND rt.code = 'CASE_HAS_DOCUMENT'))
  AND ($8::uuid IS NULL OR EXISTS (
        SELECT 1 FROM subject_relationship sr
        WHERE sr.deleted_at IS NULL
          AND sr.source_subject_id = d.id
          AND sr.target_subject_id = $8))
ORDER BY d.created_at DESC
LIMIT $9 OFFSET $10;`
