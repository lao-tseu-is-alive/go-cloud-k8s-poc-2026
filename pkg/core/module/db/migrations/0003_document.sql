-- migrate:up

-- Goéland POC — Document component (modern GED slice, spec §6.2 + expressed requirements).
--
-- A document is a first-class subject: document.id IS a subject_ref.id of kind DOCUMENT
-- (the composite FK pins the kind). Governance lives in record_metadata; typed links to
-- cases/things live in subject_relationship. This table holds the document-specific,
-- probative and search-oriented columns of a modern GED:
--   * cryptographic integrity        : sha256 (+ sha256_verified_at)
--   * no-duplication external refs    : external_system / external_id / external_url / storage_ref
--   * versioning                      : version + previous_version_id
--   * records management preparation  : is_final, is_record, status
--   * full-text search                : search_vector (generated) + GIN index

-- Controlled classification of documents -------------------------------------
CREATE TABLE document_type (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        TEXT UNIQUE NOT NULL,
    label       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    category    TEXT NOT NULL DEFAULT '', -- ENTREE, SORTIE, PLAN, DECISION, JUSTIFICATIF...
    is_active   BOOLEAN NOT NULL DEFAULT true,

    CONSTRAINT document_type_code_not_blank CHECK (length(btrim(code)) > 0)
);

-- The rich document entity ---------------------------------------------------
CREATE TABLE document (
    id                  UUID PRIMARY KEY,
    kind                TEXT NOT NULL DEFAULT 'DOCUMENT',
    document_type_id    UUID NOT NULL REFERENCES document_type (id),
    title               TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    official_date       DATE,
    storage_ref         TEXT NOT NULL DEFAULT '', -- URI: minio://..., alfresco://..., internal://...
    external_system     TEXT NOT NULL DEFAULT '', -- alfresco | minio | sharepoint | goeland-legacy
    external_id         TEXT NOT NULL DEFAULT '',
    external_url        TEXT NOT NULL DEFAULT '',
    mime_type           TEXT NOT NULL DEFAULT '',
    file_size_bytes     BIGINT NOT NULL DEFAULT 0,
    sha256              CHAR(64),
    sha256_verified_at  TIMESTAMPTZ,
    version             INT NOT NULL DEFAULT 1,
    previous_version_id UUID REFERENCES document (id),
    is_final            BOOLEAN NOT NULL DEFAULT false,
    is_record           BOOLEAN NOT NULL DEFAULT false,
    language            TEXT NOT NULL DEFAULT '',
    page_count          INT NOT NULL DEFAULT 0,
    status              SMALLINT NOT NULL DEFAULT 1, -- 1=DRAFT 2=FINAL 3=SUPERSEDED 4=ARCHIVED
    metadata            JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by          TEXT NOT NULL DEFAULT '',
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Full-text search over title + description (simple config; language-aware later).
    search_vector       TSVECTOR GENERATED ALWAYS AS (
        to_tsvector('simple', coalesce(title, '') || ' ' || coalesce(description, ''))
    ) STORED,

    -- document.id must reference a subject_ref row whose kind is DOCUMENT.
    CONSTRAINT document_subject_fkey
        FOREIGN KEY (id, kind) REFERENCES subject_ref (id, kind),
    CONSTRAINT document_kind_is_document CHECK (kind = 'DOCUMENT'),
    CONSTRAINT document_title_not_blank CHECK (length(btrim(title)) > 0),
    CONSTRAINT document_status_valid CHECK (status BETWEEN 1 AND 4),
    CONSTRAINT document_file_size_non_negative CHECK (file_size_bytes >= 0),
    CONSTRAINT document_version_positive CHECK (version >= 1)
);

CREATE INDEX idx_document_type ON document (document_type_id);
CREATE INDEX idx_document_status ON document (status);
CREATE INDEX idx_document_title ON document USING gin (to_tsvector('simple', title));
CREATE INDEX idx_document_search_vector ON document USING gin (search_vector);
CREATE INDEX idx_document_external ON document (external_system, external_id)
    WHERE external_system <> '';

-- SHA-256 is a natural dedup key when present.
CREATE UNIQUE INDEX idx_document_sha256_not_null ON document (sha256) WHERE sha256 IS NOT NULL;

-- Keep updated_at correct even for writes outside the service code.
-- migrate:statementbegin
CREATE OR REPLACE FUNCTION set_document_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- migrate:statementend

CREATE TRIGGER trg_document_set_updated_at
    BEFORE UPDATE ON document
    FOR EACH ROW
    EXECUTE FUNCTION set_document_updated_at();

-- migrate:down

DROP TRIGGER IF EXISTS trg_document_set_updated_at ON document;
DROP FUNCTION IF EXISTS set_document_updated_at();
DROP TABLE IF EXISTS document;
DROP TABLE IF EXISTS document_type;
