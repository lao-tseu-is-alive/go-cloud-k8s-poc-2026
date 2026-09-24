-- migrate:up

-- Goéland POC — Case component (spec v1 §6.1, spec v2 §24, roadmap GLD-011).
--
-- A case (affaire) is a first-class subject: case_file.id IS a subject_ref.id of
-- kind CASE (the composite FK pins the kind). Governance lives in record_metadata;
-- participants (actors), documents, things and related cases are typed
-- subject_relationship edges (CASE_HAS_ACTOR_*, CASE_HAS_DOCUMENT,
-- CASE_CONCERNS_THING, CASE_PARENT_OF_CASE), never columns here.
-- A case exists independently of any workflow (CASE != WORKFLOW_INSTANCE).
--
-- Lifecycle (IMPLEMENTATION_STATUS §3g): status holds the operational state,
-- soft deletion stays in record_metadata.deleted_at, and ARCHIVED / DISPOSED
-- belong to the future retention tables (GLD-032).

-- Controlled classification of cases ---------------------------------------------
CREATE TABLE case_type (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code                   TEXT UNIQUE NOT NULL,
    label                  TEXT NOT NULL DEFAULT '',
    description            TEXT NOT NULL DEFAULT '',
    -- When set, CreateCase allocates the business reference in this namespace.
    business_ref_namespace TEXT NOT NULL DEFAULT '',
    is_active              BOOLEAN NOT NULL DEFAULT true,

    CONSTRAINT case_type_code_not_blank CHECK (length(btrim(code)) > 0),
    CONSTRAINT case_type_namespace_format CHECK (business_ref_namespace ~ '^([A-Z][A-Z0-9_]{0,31})?$')
);

-- The case entity -------------------------------------------------------------------
CREATE TABLE case_file (
    id             UUID PRIMARY KEY,
    kind           TEXT NOT NULL DEFAULT 'CASE',
    case_type_id   UUID NOT NULL REFERENCES case_type (id),
    title          TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    status         SMALLINT NOT NULL DEFAULT 1, -- 1=OPEN 2=IN_PROGRESS 3=SUSPENDED 4=CLOSED
    opened_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at      TIMESTAMPTZ,
    closed_by      TEXT NOT NULL DEFAULT '',
    closure_reason TEXT NOT NULL DEFAULT '',
    metadata       JSONB NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by     TEXT NOT NULL DEFAULT '',
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Accent-insensitive full-text search over title + description (same
    -- immutable_unaccent wrapper as documents and actors, migration 0005).
    search_vector  TSVECTOR GENERATED ALWAYS AS (
        to_tsvector('simple', immutable_unaccent(coalesce(title, '') || ' ' || coalesce(description, '')))
    ) STORED,

    CONSTRAINT case_file_subject_fkey FOREIGN KEY (id, kind) REFERENCES subject_ref (id, kind),
    CONSTRAINT case_file_kind_is_case CHECK (kind = 'CASE'),
    CONSTRAINT case_file_title_not_blank CHECK (length(btrim(title)) > 0),
    CONSTRAINT case_file_status_valid CHECK (status BETWEEN 1 AND 4),
    -- Closure stamps exist exactly when the case is closed.
    CONSTRAINT case_file_closure_consistent CHECK (
        (status = 4) = (closed_at IS NOT NULL)
        AND (status = 4 OR (closed_by = '' AND closure_reason = ''))
    )
);

CREATE INDEX idx_case_file_type ON case_file (case_type_id);
CREATE INDEX idx_case_file_status ON case_file (status);
CREATE INDEX idx_case_file_search_vector ON case_file USING gin (search_vector);

-- Keep updated_at correct even for writes outside the service code.
-- migrate:statementbegin
CREATE OR REPLACE FUNCTION set_case_file_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- migrate:statementend

CREATE TRIGGER trg_case_file_set_updated_at
    BEFORE UPDATE ON case_file
    FOR EACH ROW
    EXECUTE FUNCTION set_case_file_updated_at();

-- Seed case types (spec v1 §14.1) ---------------------------------------------------
INSERT INTO case_type (code, label, description, business_ref_namespace) VALUES
    ('OPC_DEMANDE_PC',  'Demande de permis de construire', 'Demande de permis de construire (OPC)', 'OPC'),
    ('GENERIC_REQUEST', 'Demande générique',               'Demande générique sans procédure dédiée', 'GEN')
ON CONFLICT (code) DO NOTHING;

-- Expanded case roles and case-to-case links (spec v2 §5.6, §6) --------------------
-- Requester and mandatee were seeded by 0004; these complete the first role set.
INSERT INTO relationship_type (code, label, source_kind, target_kind, inverse_label, description, is_directed) VALUES
    ('CASE_HAS_ACTOR_OWNER',      'Affaire a propriétaire', 'CASE', 'ACTOR', 'Acteur propriétaire dans affaire', 'Acteur propriétaire de l''objet concerné', true),
    ('CASE_HAS_ACTOR_ARCHITECT',  'Affaire a architecte',   'CASE', 'ACTOR', 'Acteur architecte dans affaire',   'Bureau d''architectes ou architecte du projet', true),
    ('CASE_HAS_ACTOR_CONTRACTOR', 'Affaire a entreprise',   'CASE', 'ACTOR', 'Acteur entreprise dans affaire',   'Entreprise exécutant les travaux', true),
    ('CASE_RELATED_TO_CASE',      'Affaire liée à affaire', 'CASE', 'CASE',  'Affaire liée à affaire',           'Lien non hiérarchique entre affaires', false)
ON CONFLICT (code) DO NOTHING;

-- migrate:down

DELETE FROM relationship_type WHERE code IN
    ('CASE_HAS_ACTOR_OWNER','CASE_HAS_ACTOR_ARCHITECT','CASE_HAS_ACTOR_CONTRACTOR','CASE_RELATED_TO_CASE');

DROP TRIGGER IF EXISTS trg_case_file_set_updated_at ON case_file;
DROP FUNCTION IF EXISTS set_case_file_updated_at();
DROP TABLE IF EXISTS case_file;
DROP TABLE IF EXISTS case_type;
