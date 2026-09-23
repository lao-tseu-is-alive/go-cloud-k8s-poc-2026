-- migrate:up

-- Goéland POC — Document / DocumentVersion / ContentBlob split (spec v2 §15-22,
-- roadmap GLD-023). Additive phase: new tables + lossless backfill. Migration
-- 0009 drops the superseded document columns.
--
--   DOCUMENT          logical business document (identity, type, title, context)
--     └─ DOCUMENT_VERSION   a dated state of the document (append-only)
--          └─ CONTENT_BLOB   the binary content, identified by its SHA-256
--
-- SHA-256 is the identity of the CONTENT, not of the document: it is unique on
-- content_blob, so identical bytes are stored once and may back several versions
-- (v2 §19). A version without content (content_blob_id NULL) is a metadata-only
-- document or an external reference (external_system / external_id / external_url
-- stay on document: decided enhancement, IMPLEMENTATION_STATUS §3). A blob with an
-- empty storage_ref is content known by its digest whose bytes Goéland does not hold.

CREATE TABLE content_blob (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sha256          CHAR(64) NOT NULL,
    storage_ref     TEXT NOT NULL DEFAULT '', -- internal://..., s3://...; '' = bytes held elsewhere
    mime_type       TEXT NOT NULL DEFAULT '',
    file_size_bytes BIGINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by      TEXT NOT NULL DEFAULT '',
    verified_at     TIMESTAMPTZ,              -- last probative re-verification (GLD-021)

    CONSTRAINT content_blob_sha256_unique UNIQUE (sha256),
    CONSTRAINT content_blob_sha256_lower_hex CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT content_blob_size_non_negative CHECK (file_size_bytes >= 0)
);

CREATE TABLE document_version (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id     UUID NOT NULL REFERENCES document (id),
    version_no      INT NOT NULL,
    content_blob_id UUID REFERENCES content_blob (id),
    page_count      INT NOT NULL DEFAULT 0,
    is_final        BOOLEAN NOT NULL DEFAULT false,
    is_record       BOOLEAN NOT NULL DEFAULT false,
    validated_at    TIMESTAMPTZ,
    validated_by    TEXT NOT NULL DEFAULT '',
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by      TEXT NOT NULL DEFAULT '',

    CONSTRAINT document_version_no_unique UNIQUE (document_id, version_no),
    CONSTRAINT document_version_no_positive CHECK (version_no >= 1),
    CONSTRAINT document_version_page_count_non_negative CHECK (page_count >= 0),
    CONSTRAINT document_version_final_is_validated CHECK (NOT is_final OR validated_at IS NOT NULL),
    -- Declaring a record freezes the version: a record is final (and so validated).
    CONSTRAINT document_version_record_is_final CHECK (NOT is_record OR is_final)
);

CREATE INDEX idx_document_version_document ON document_version (document_id, version_no DESC);
CREATE INDEX idx_document_version_blob ON document_version (content_blob_id);

-- The current version is explicit (IMPLEMENTATION_STATUS §3g), never max(version_no).
ALTER TABLE document
    ADD COLUMN current_version_id UUID REFERENCES document_version (id);

-- Versions are append-only; a validated (final) or record version is immutable,
-- and no version is ever physically deleted (v2 §18, §35). A still-mutable
-- version may be validated or have its page count / metadata corrected, but its
-- document, number, content and creation stamps never change.
-- migrate:statementbegin
CREATE OR REPLACE FUNCTION guard_document_version() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'document_version rows are never deleted (version %)', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    IF OLD.validated_at IS NOT NULL OR OLD.is_record THEN
        RAISE EXCEPTION 'document_version % is validated or a record and is immutable', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    IF NEW.document_id <> OLD.document_id OR NEW.version_no <> OLD.version_no
        OR NEW.content_blob_id IS DISTINCT FROM OLD.content_blob_id
        OR NEW.created_at <> OLD.created_at OR NEW.created_by <> OLD.created_by THEN
        RAISE EXCEPTION 'document_version % identity and content are immutable', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- migrate:statementend

CREATE TRIGGER trg_document_version_guard
    BEFORE UPDATE OR DELETE ON document_version
    FOR EACH ROW
    EXECUTE FUNCTION guard_document_version();

-- Backfill (lossless): one content_blob per distinct registered digest, one
-- version per existing document carrying its file, version and records fields.
INSERT INTO content_blob (sha256, storage_ref, mime_type, file_size_bytes, created_at, created_by, verified_at)
SELECT DISTINCT ON (lower(d.sha256))
       lower(d.sha256), d.storage_ref, d.mime_type, d.file_size_bytes, d.created_at, d.created_by, d.sha256_verified_at
FROM document d
WHERE d.sha256 IS NOT NULL
ORDER BY lower(d.sha256), d.created_at;

INSERT INTO document_version (document_id, version_no, content_blob_id, page_count, is_final, is_record,
                              validated_at, validated_by, created_at, created_by)
SELECT d.id, d.version, cb.id, d.page_count, d.is_final OR d.is_record, d.is_record,
       CASE WHEN d.is_final OR d.is_record THEN d.updated_at END,
       CASE WHEN d.is_final OR d.is_record THEN d.created_by ELSE '' END,
       d.created_at, d.created_by
FROM document d
LEFT JOIN content_blob cb ON cb.sha256 = lower(d.sha256);

-- Keep updated_at untouched: linking the backfilled version is not a user edit.
ALTER TABLE document DISABLE TRIGGER trg_document_set_updated_at;
-- The document status follows its (backfilled) current version.
UPDATE document d
SET current_version_id = v.id,
    status = CASE WHEN v.is_final AND d.status = 1 THEN 2 ELSE d.status END
FROM document_version v
WHERE v.document_id = d.id;
ALTER TABLE document ENABLE TRIGGER trg_document_set_updated_at;

-- migrate:down

ALTER TABLE document DROP COLUMN IF EXISTS current_version_id;
DROP TRIGGER IF EXISTS trg_document_version_guard ON document_version;
DROP FUNCTION IF EXISTS guard_document_version();
DROP TABLE IF EXISTS document_version;
DROP TABLE IF EXISTS content_blob;
