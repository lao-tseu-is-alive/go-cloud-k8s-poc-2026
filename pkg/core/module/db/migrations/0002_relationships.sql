-- migrate:up

-- Goéland POC — typed relationships between subjects (spec §7).
--
-- The heart of the contextual model: subjects are linked by explicit, typed,
-- validated edges. relationship_type is the catalogue of allowed edges (with
-- source/target kind constraints); subject_relationship holds the actual edges.
--
-- Application rule (enforced in the service layer, not by SQL alone):
--   before creating an edge the service checks that source & target exist, the
--   type exists, source.kind == type.source_kind, target.kind == type.target_kind,
--   the caller is authorised, and no identical active edge already exists.

-- Catalogue of allowed typed relations ---------------------------------------
CREATE TABLE relationship_type (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code          TEXT UNIQUE NOT NULL,
    label         TEXT NOT NULL DEFAULT '',
    source_kind   TEXT NOT NULL REFERENCES subject_kind (code),
    target_kind   TEXT NOT NULL REFERENCES subject_kind (code),
    is_directed   BOOLEAN NOT NULL DEFAULT true,
    inverse_label TEXT NOT NULL DEFAULT '',
    description   TEXT NOT NULL DEFAULT '',
    is_active     BOOLEAN NOT NULL DEFAULT true,

    CONSTRAINT relationship_type_code_not_blank CHECK (length(btrim(code)) > 0)
);

-- Actual edges in the business graph -----------------------------------------
CREATE TABLE subject_relationship (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_subject_id    UUID NOT NULL REFERENCES subject_ref (id),
    target_subject_id    UUID NOT NULL REFERENCES subject_ref (id),
    relationship_type_id UUID NOT NULL REFERENCES relationship_type (id),
    role_detail          TEXT NOT NULL DEFAULT '',
    valid_from           TIMESTAMPTZ,
    valid_to             TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by           TEXT NOT NULL DEFAULT '',
    deleted_at           TIMESTAMPTZ,
    deleted_by           TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_subject_relationship_source ON subject_relationship (source_subject_id);
CREATE INDEX idx_subject_relationship_target ON subject_relationship (target_subject_id);
CREATE INDEX idx_subject_relationship_type ON subject_relationship (relationship_type_id);

-- Uniqueness of an *active* edge (soft-deleted edges may be recreated later).
CREATE UNIQUE INDEX idx_subject_relationship_active_unique
    ON subject_relationship (source_subject_id, target_subject_id, relationship_type_id)
    WHERE deleted_at IS NULL;

-- migrate:down

DROP TABLE IF EXISTS subject_relationship;
DROP TABLE IF EXISTS relationship_type;
