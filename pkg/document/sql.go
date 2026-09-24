package document

// SQL fragments for the document repository. Column projections are the single
// source of truth for pgx named scanning (columns map to `db` tags).
//
// Queries use pgx v5 named parameters (@name) bound through pgx.NamedArgs at the
// call sites. Named binding keeps the argument list self-documenting and lets the
// SQL and the Go call site be read side by side without counting positions.

const documentColumns = `
d.id, d.document_type_id, d.title, d.description, d.official_date,
d.external_system, d.external_id, d.external_url, d.language, d.status, d.metadata,
d.current_version_id, d.created_at, d.created_by, d.updated_at`

const insertDocumentSQL = `
INSERT INTO document AS d (
    id, document_type_id, title, description, official_date,
    external_system, external_id, external_url, language, status, metadata, created_by
) VALUES (
    @id, @document_type_id, @title, @description, @official_date,
    @external_system, @external_id, @external_url, @language, @status, @metadata, @created_by
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

// setCurrentVersionSQL points a document at its new current version and aligns
// its status (FINAL when that version is final, DRAFT otherwise).
const setCurrentVersionSQL = `
UPDATE document d
SET current_version_id = @version_id, status = @status
WHERE d.id = @id
RETURNING ` + documentColumns + `;`

const setDocumentStatusSQL = `
UPDATE document d
SET status = @status
WHERE d.id = @id
RETURNING ` + documentColumns + `;`

// findReusableDocumentSQL finds the oldest live (not soft-deleted) document with
// a version backed by the blob: the target of automatic reuse (spec v2 §20).
const findReusableDocumentSQL = `
SELECT ` + documentColumns + `
FROM document d
JOIN record_metadata rm ON rm.subject_id = d.id
WHERE rm.deleted_at IS NULL
  AND EXISTS (SELECT 1 FROM document_version v WHERE v.document_id = d.id AND v.content_blob_id = @content_blob_id)
ORDER BY d.created_at
LIMIT 1;`

// --- document_version --------------------------------------------------------

const versionColumns = `
v.id, v.document_id, v.version_no, v.content_blob_id, v.page_count, v.is_final,
v.is_record, v.validated_at, v.validated_by, v.metadata, v.created_at, v.created_by`

// insertVersionSQL appends the next version of a document. Callers hold the
// document's record_metadata row lock (EnsureMutableTx), which serializes
// concurrent additions, so max(version_no) + 1 cannot collide.
const insertVersionSQL = `
INSERT INTO document_version AS v (
    document_id, version_no, content_blob_id, page_count, is_final, is_record,
    validated_at, validated_by, metadata, created_by
) VALUES (
    @document_id,
    (SELECT coalesce(max(version_no), 0) + 1 FROM document_version WHERE document_id = @document_id),
    @content_blob_id, @page_count, @is_final, @is_record,
    CASE WHEN @is_final::boolean THEN now() END,
    CASE WHEN @is_final::boolean THEN @created_by ELSE '' END,
    @metadata, @created_by
)
RETURNING ` + versionColumns + `;`

const getVersionSQL = `
SELECT ` + versionColumns + `
FROM document_version v
WHERE v.id = @id;`

const listVersionsSQL = `
SELECT ` + versionColumns + `
FROM document_version v
WHERE v.document_id = @document_id
ORDER BY v.version_no DESC;`

// finalizeCurrentVersionSQL validates the current version once; zero rows means
// it was already final (finalizing is then idempotent).
const finalizeCurrentVersionSQL = `
UPDATE document_version v
SET is_final = true, validated_at = now(), validated_by = @operator_id
WHERE v.id = (SELECT current_version_id FROM document WHERE id = @document_id)
  AND v.validated_at IS NULL
RETURNING ` + versionColumns + `;`

// --- content_blob ------------------------------------------------------------

const blobColumns = `
b.id, b.sha256, b.storage_ref, b.mime_type, b.file_size_bytes, b.created_at, b.created_by, b.verified_at`

// insertBlobSQL registers content unless its digest is already known; zero rows
// means another blob holds the digest (see getBlobBySHA256SQL).
const insertBlobSQL = `
INSERT INTO content_blob AS b (sha256, storage_ref, mime_type, file_size_bytes, created_by)
VALUES (@sha256, @storage_ref, @mime_type, @file_size_bytes, @created_by)
ON CONFLICT (sha256) DO NOTHING
RETURNING ` + blobColumns + `;`

const getBlobBySHA256SQL = `
SELECT ` + blobColumns + `
FROM content_blob b
WHERE b.sha256 = @sha256;`

const getBlobSQL = `
SELECT ` + blobColumns + `
FROM content_blob b
WHERE b.id = @id;`

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
LEFT JOIN document_version cv ON cv.id = d.current_version_id
WHERE (@query = '' OR d.search_vector @@ plainto_tsquery('simple', immutable_unaccent(@query)))
  AND (@document_type_code = '' OR d.document_type_id = (SELECT id FROM document_type WHERE code = @document_type_code))
  AND rm.confidentiality_level <= @confidentiality_max
  AND (NOT @only_records OR cv.is_record)
  AND (NOT @only_final OR cv.is_final)
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

// --- document_type administration (GLD-040) ---------------------------------------------

const insertDocumentTypeSQL = `
INSERT INTO document_type (code, label, description, category)
VALUES (@code, @label, @description, @category)
RETURNING ` + documentTypeColumns + `;`

const getDocumentTypeForUpdateSQL = `
SELECT ` + documentTypeColumns + `
FROM document_type
WHERE code = @code
FOR UPDATE;`

// updateDocumentTypeSQL replaces the fields given (NULL keeps the current value); the
// code is immutable.
const updateDocumentTypeSQL = `
UPDATE document_type
SET label = coalesce(@label::text, label),
    description = coalesce(@description::text, description),
    category = coalesce(@category::text, category),
    is_active = coalesce(@is_active::boolean, is_active)
WHERE code = @code
RETURNING ` + documentTypeColumns + `;`
