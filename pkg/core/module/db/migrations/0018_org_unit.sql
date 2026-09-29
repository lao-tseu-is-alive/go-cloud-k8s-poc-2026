-- migrate:up

-- Goéland POC — internal organizational units (spec v2 §5.7, §31, roadmap GLD-041).
--
-- An ORG_UNIT is an internal unit of the administration (direction, service,
-- office, ...), never an ACTOR. org_unit.id IS a subject_ref.id of kind ORG_UNIT,
-- so tasks, circulations and cases can target a unit through typed
-- relationships and record_metadata.owner_org_id can name one.
--
-- Modelled from the production structure (aggregates only): one tree, up to 7
-- levels, 7 official types, dissolved units kept for history. A unit is never
-- deleted: it is dissolved (with a reason) and stays visible as history.
--
-- There is no natural unique code: the abbreviation names the service a unit
-- belongs to and is inherited by most sub-units (111 distinct values for 738
-- units), and labels repeat across services ("Secrétariat"). Identity is the
-- subject UUID; live siblings never share a label; external_ref keeps the id
-- of a source system (import idempotency).

-- Controlled classification, administrable (GLD-040) --------------------------------
CREATE TABLE org_unit_type (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        TEXT UNIQUE NOT NULL,
    label       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    -- Display order, from the top of the hierarchy down; new types come last.
    sort_order  INT NOT NULL DEFAULT 100,
    is_active   BOOLEAN NOT NULL DEFAULT true,

    CONSTRAINT org_unit_type_code_not_blank CHECK (length(btrim(code)) > 0)
);

INSERT INTO org_unit_type (code, label, description, sort_order) VALUES
    ('ENTERPRISE', 'Entreprise', 'Racine de l''organisation (la Ville)', 10),
    ('DIRECTION',  'Direction',  'Direction municipale', 20),
    ('SERVICE',    'Service',    'Service d''une direction', 30),
    ('OFFICE',     'Office',     'Office', 40),
    ('DIVISION',   'Division',   'Division', 50),
    ('BUREAU',     'Bureau',     'Bureau', 60),
    ('UNIT',       'Unité',      'Unité', 70)
ON CONFLICT (code) DO NOTHING;

-- The unit entity ----------------------------------------------------------------------
CREATE TABLE org_unit (
    id                 UUID PRIMARY KEY,
    kind               TEXT NOT NULL DEFAULT 'ORG_UNIT',
    org_unit_type_id   UUID NOT NULL REFERENCES org_unit_type (id),
    -- The abbreviation (sigle) used across the administration, e.g. SOI; often
    -- shared with the parent unit, so not unique.
    abbreviation       TEXT NOT NULL DEFAULT '',
    label              TEXT NOT NULL,
    description        TEXT NOT NULL DEFAULT '',
    -- Functional mailbox of the unit (not a person's address).
    email              TEXT NOT NULL DEFAULT '',
    parent_id          UUID REFERENCES org_unit (id),
    -- Identifier in a source system (e.g. goeland:1234); set at creation only.
    external_ref       TEXT NOT NULL DEFAULT '',
    dissolved_at       TIMESTAMPTZ,
    dissolved_by       TEXT NOT NULL DEFAULT '',
    dissolution_reason TEXT NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by         TEXT NOT NULL DEFAULT '',
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),

    search_vector TSVECTOR GENERATED ALWAYS AS (
        to_tsvector('simple', immutable_unaccent(
            coalesce(abbreviation, '') || ' ' || coalesce(label, '') || ' ' || coalesce(description, '')))
    ) STORED,

    CONSTRAINT org_unit_subject_fkey FOREIGN KEY (id, kind) REFERENCES subject_ref (id, kind),
    CONSTRAINT org_unit_kind_is_org_unit CHECK (kind = 'ORG_UNIT'),
    CONSTRAINT org_unit_label_not_blank CHECK (length(btrim(label)) > 0),
    CONSTRAINT org_unit_not_own_parent CHECK (parent_id <> id),
    -- Dissolution stamps exist exactly when the unit is dissolved, with a reason.
    CONSTRAINT org_unit_dissolution_consistent CHECK (
        (dissolved_at IS NULL AND dissolved_by = '' AND dissolution_reason = '')
        OR (dissolved_at IS NOT NULL AND length(btrim(dissolution_reason)) > 0)
    )
);

-- Live siblings never share a label, whatever its case (roots are siblings).
CREATE UNIQUE INDEX idx_org_unit_sibling_label_unique
    ON org_unit (coalesce(parent_id, '00000000-0000-0000-0000-000000000000'::uuid), lower(label))
    WHERE dissolved_at IS NULL;
CREATE UNIQUE INDEX idx_org_unit_external_ref_unique ON org_unit (external_ref) WHERE external_ref <> '';
CREATE INDEX idx_org_unit_abbreviation ON org_unit (lower(abbreviation));
CREATE INDEX idx_org_unit_parent ON org_unit (parent_id);
CREATE INDEX idx_org_unit_type ON org_unit (org_unit_type_id);
CREATE INDEX idx_org_unit_search_vector ON org_unit USING gin (search_vector);

-- updated_at is kept by the database; the tree never contains a cycle and the
-- external reference never changes once set (the service checks first, the
-- trigger guarantees it).
-- migrate:statementbegin
CREATE OR REPLACE FUNCTION guard_org_unit() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        IF OLD.external_ref <> '' AND NEW.external_ref <> OLD.external_ref THEN
            RAISE EXCEPTION 'org_unit % external_ref is immutable', OLD.id
                USING ERRCODE = 'integrity_constraint_violation';
        END IF;
        NEW.updated_at := now();
    END IF;
    IF NEW.parent_id IS NOT NULL AND EXISTS (
        WITH RECURSIVE ancestors AS (
            SELECT id, parent_id FROM org_unit WHERE id = NEW.parent_id
            UNION
            SELECT o.id, o.parent_id FROM org_unit o JOIN ancestors a ON o.id = a.parent_id
        )
        SELECT 1 FROM ancestors WHERE id = NEW.id
    ) THEN
        RAISE EXCEPTION 'org_unit % cannot be placed under its own descendant', NEW.id
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- migrate:statementend

CREATE TRIGGER trg_guard_org_unit
    BEFORE INSERT OR UPDATE ON org_unit
    FOR EACH ROW
    EXECUTE FUNCTION guard_org_unit();

-- Governance: the owning unit becomes a typed reference ---------------------------------
-- owner_org_id was free text; no org_unit existed before this migration, so no
-- previous value can reference one and every value becomes NULL.
DROP INDEX IF EXISTS idx_record_metadata_owner_org;
ALTER TABLE record_metadata ALTER COLUMN owner_org_id DROP DEFAULT;
ALTER TABLE record_metadata ALTER COLUMN owner_org_id DROP NOT NULL;
ALTER TABLE record_metadata ALTER COLUMN owner_org_id TYPE UUID USING NULL;
ALTER TABLE record_metadata
    ADD CONSTRAINT record_metadata_owner_org_fkey FOREIGN KEY (owner_org_id) REFERENCES org_unit (id);
CREATE INDEX idx_record_metadata_owner_org ON record_metadata (owner_org_id);

-- Unit roles in a case (the three production roles covering 92% of the links) --------
INSERT INTO relationship_type (code, label, source_kind, target_kind, inverse_label, description, is_directed) VALUES
    ('CASE_HAS_ORG_UNIT_LEADER',      'Affaire pilotée par unité',     'CASE', 'ORG_UNIT', 'Unité pilote de',       'Unité qui pilote l''affaire (Leader)', true),
    ('CASE_HAS_ORG_UNIT_MANAGER',     'Affaire gérée par unité',       'CASE', 'ORG_UNIT', 'Unité gestionnaire de', 'Unité gestionnaire du dossier', true),
    ('CASE_HAS_ORG_UNIT_PARTICIPANT', 'Affaire avec unité participante', 'CASE', 'ORG_UNIT', 'Unité participante à', 'Unité qui participe au traitement', true)
ON CONFLICT (code) DO NOTHING;

-- The type catalogue is administered like the others (GLD-040).
ALTER TABLE reference_change DROP CONSTRAINT reference_change_catalogue_valid;
ALTER TABLE reference_change ADD CONSTRAINT reference_change_catalogue_valid CHECK (
    catalogue IN ('case_type', 'relationship_type', 'organization_category', 'document_type', 'thing_type', 'org_unit_type')
);

-- migrate:down

ALTER TABLE reference_change DROP CONSTRAINT reference_change_catalogue_valid;
ALTER TABLE reference_change ADD CONSTRAINT reference_change_catalogue_valid CHECK (
    catalogue IN ('case_type', 'relationship_type', 'organization_category', 'document_type', 'thing_type')
);

DELETE FROM relationship_type WHERE code IN
    ('CASE_HAS_ORG_UNIT_LEADER', 'CASE_HAS_ORG_UNIT_MANAGER', 'CASE_HAS_ORG_UNIT_PARTICIPANT');

ALTER TABLE record_metadata DROP CONSTRAINT IF EXISTS record_metadata_owner_org_fkey;
DROP INDEX IF EXISTS idx_record_metadata_owner_org;
ALTER TABLE record_metadata ALTER COLUMN owner_org_id TYPE TEXT USING coalesce(owner_org_id::text, '');
ALTER TABLE record_metadata ALTER COLUMN owner_org_id SET DEFAULT '';
ALTER TABLE record_metadata ALTER COLUMN owner_org_id SET NOT NULL;
CREATE INDEX idx_record_metadata_owner_org ON record_metadata (owner_org_id);

DROP TRIGGER IF EXISTS trg_guard_org_unit ON org_unit;
DROP FUNCTION IF EXISTS guard_org_unit();
DROP TABLE IF EXISTS org_unit;
DROP TABLE IF EXISTS org_unit_type;
