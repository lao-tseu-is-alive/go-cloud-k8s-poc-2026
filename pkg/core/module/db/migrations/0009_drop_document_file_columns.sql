-- migrate:up

-- Goéland POC — finish the Document / DocumentVersion / ContentBlob split (spec
-- v2 §21 phase E, roadmap GLD-023). The file, version and records columns moved
-- to content_blob / document_version in migration 0008 and are dropped here.
-- previous_version_id is superseded by versions inside one document; the
-- DOCUMENT_PREVIOUS_VERSION relationships it produced are kept as graph edges.
-- Dropping sha256 also drops its unique index: uniqueness now lives on
-- content_blob.sha256.

ALTER TABLE document
    DROP CONSTRAINT IF EXISTS document_file_size_non_negative,
    DROP CONSTRAINT IF EXISTS document_version_positive,
    DROP COLUMN storage_ref,
    DROP COLUMN mime_type,
    DROP COLUMN file_size_bytes,
    DROP COLUMN sha256,
    DROP COLUMN sha256_verified_at,
    DROP COLUMN version,
    DROP COLUMN previous_version_id,
    DROP COLUMN is_final,
    DROP COLUMN is_record,
    DROP COLUMN page_count;

-- migrate:down

-- Best-effort restore from the current version (the history of older versions
-- has no place in the pre-split model).
ALTER TABLE document
    ADD COLUMN storage_ref         TEXT NOT NULL DEFAULT '',
    ADD COLUMN mime_type           TEXT NOT NULL DEFAULT '',
    ADD COLUMN file_size_bytes     BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN sha256              CHAR(64),
    ADD COLUMN sha256_verified_at  TIMESTAMPTZ,
    ADD COLUMN version             INT NOT NULL DEFAULT 1,
    ADD COLUMN previous_version_id UUID REFERENCES document (id),
    ADD COLUMN is_final            BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN is_record           BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN page_count          INT NOT NULL DEFAULT 0,
    ADD CONSTRAINT document_file_size_non_negative CHECK (file_size_bytes >= 0),
    ADD CONSTRAINT document_version_positive CHECK (version >= 1);

ALTER TABLE document DISABLE TRIGGER trg_document_set_updated_at;
UPDATE document d
SET storage_ref = coalesce(cb.storage_ref, ''),
    mime_type = coalesce(cb.mime_type, ''),
    file_size_bytes = coalesce(cb.file_size_bytes, 0),
    sha256 = cb.sha256,
    sha256_verified_at = cb.verified_at,
    version = v.version_no,
    is_final = v.is_final,
    is_record = v.is_record,
    page_count = v.page_count
FROM document_version v
LEFT JOIN content_blob cb ON cb.id = v.content_blob_id
WHERE v.id = d.current_version_id;
ALTER TABLE document ENABLE TRIGGER trg_document_set_updated_at;

-- A blob shared by several documents cannot satisfy the old per-document unique
-- index; keep the digest on the oldest document only.
UPDATE document d
SET sha256 = NULL
WHERE d.sha256 IS NOT NULL
  AND EXISTS (SELECT 1 FROM document o WHERE o.sha256 = d.sha256 AND o.created_at < d.created_at);

CREATE UNIQUE INDEX idx_document_sha256_not_null ON document (sha256) WHERE sha256 IS NOT NULL;
