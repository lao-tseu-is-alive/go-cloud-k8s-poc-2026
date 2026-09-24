-- migrate:up

-- Goéland POC — Thing component (spec v1 §6.3, spec v2 §25, roadmap GLD-016).
--
-- A thing is a business object, often territorial and georeferenced: a parcel,
-- a building, a street, a tree... thing.id IS a subject_ref.id of kind THING.
-- Geometry is typed and in the Swiss national frame (EPSG:2056, LV95) with a
-- GIST index (IMPLEMENTATION_STATUS §3g). Parcels and buildings carry their
-- official identifiers in 1:1 specialization tables. Cases, documents and
-- actors attach through typed relationships (CASE_CONCERNS_THING,
-- DOCUMENT_REPRESENTS_THING, THING_HAS_ACTOR_*), never columns here.

-- Controlled classification; specialization picks the detail table:
-- 0 = generic, 1 = parcel (thing_parcel), 2 = building (thing_building).
CREATE TABLE thing_type (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code           TEXT UNIQUE NOT NULL,
    label          TEXT NOT NULL DEFAULT '',
    description    TEXT NOT NULL DEFAULT '',
    specialization SMALLINT NOT NULL DEFAULT 0,
    is_active      BOOLEAN NOT NULL DEFAULT true,

    CONSTRAINT thing_type_code_not_blank CHECK (length(btrim(code)) > 0),
    CONSTRAINT thing_type_specialization_valid CHECK (specialization BETWEEN 0 AND 2)
);

CREATE TABLE thing (
    id            UUID PRIMARY KEY,
    kind          TEXT NOT NULL DEFAULT 'THING',
    thing_type_id UUID NOT NULL REFERENCES thing_type (id),
    name          TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    external_ref  TEXT NOT NULL DEFAULT '', -- identifier in a source system (e.g. a GIS layer)
    geom          geometry(Geometry, 2056),
    metadata      JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by    TEXT NOT NULL DEFAULT '',
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    search_vector TSVECTOR GENERATED ALWAYS AS (
        to_tsvector('simple', immutable_unaccent(
            coalesce(name, '') || ' ' || coalesce(description, '') || ' ' || coalesce(external_ref, '')))
    ) STORED,

    CONSTRAINT thing_subject_fkey FOREIGN KEY (id, kind) REFERENCES subject_ref (id, kind),
    CONSTRAINT thing_kind_is_thing CHECK (kind = 'THING'),
    CONSTRAINT thing_name_not_blank CHECK (length(btrim(name)) > 0),
    CONSTRAINT thing_geom_valid CHECK (geom IS NULL OR ST_IsValid(geom))
);

CREATE INDEX idx_thing_type ON thing (thing_type_id);
CREATE INDEX idx_thing_geom ON thing USING gist (geom);
CREATE INDEX idx_thing_search_vector ON thing USING gin (search_vector);

-- migrate:statementbegin
CREATE OR REPLACE FUNCTION set_thing_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- migrate:statementend

CREATE TRIGGER trg_thing_set_updated_at
    BEFORE UPDATE ON thing
    FOR EACH ROW
    EXECUTE FUNCTION set_thing_updated_at();

-- Parcel (bien-fonds): commune OFS number + parcel number, federal EGRID.
CREATE TABLE thing_parcel (
    thing_id      UUID PRIMARY KEY REFERENCES thing (id),
    commune_ofs   INTEGER NOT NULL,
    parcel_number TEXT NOT NULL,
    egrid         TEXT NOT NULL DEFAULT '',
    surface_m2    NUMERIC(14, 2), -- registered surface (the geometry area is computed)

    CONSTRAINT thing_parcel_commune_valid CHECK (commune_ofs BETWEEN 1 AND 9999),
    CONSTRAINT thing_parcel_number_not_blank CHECK (length(btrim(parcel_number)) > 0),
    CONSTRAINT thing_parcel_egrid_format CHECK (egrid = '' OR egrid ~ '^CH[0-9A-Z]{12}$'),
    CONSTRAINT thing_parcel_surface_positive CHECK (surface_m2 IS NULL OR surface_m2 > 0),
    CONSTRAINT thing_parcel_number_unique UNIQUE (commune_ofs, parcel_number)
);
CREATE UNIQUE INDEX idx_thing_parcel_egrid ON thing_parcel (egrid) WHERE egrid <> '';

-- Building: federal EGID (RegBL/GWR), cantonal ECA number, status.
CREATE TABLE thing_building (
    thing_id          UUID PRIMARY KEY REFERENCES thing (id),
    egid              INTEGER,
    eca_number        TEXT NOT NULL DEFAULT '',
    construction_year SMALLINT,
    building_status   SMALLINT NOT NULL DEFAULT 0, -- mirrors the BuildingStatus proto enum

    CONSTRAINT thing_building_egid_positive CHECK (egid IS NULL OR egid > 0),
    CONSTRAINT thing_building_year_valid CHECK (construction_year IS NULL OR construction_year BETWEEN 1000 AND 2100),
    CONSTRAINT thing_building_status_valid CHECK (building_status BETWEEN 0 AND 7)
);
CREATE UNIQUE INDEX idx_thing_building_egid ON thing_building (egid) WHERE egid IS NOT NULL;

INSERT INTO thing_type (code, label, description, specialization) VALUES
    ('PARCEL',         'Parcelle',       'Bien-fonds du registre foncier', 1),
    ('BUILDING',       'Bâtiment',       'Bâtiment du registre des bâtiments (RegBL)', 2),
    ('STREET',         'Rue',            'Voie publique ou privée', 0),
    ('TREE',           'Arbre',          'Arbre ou élément végétal', 0),
    ('INFRASTRUCTURE', 'Infrastructure', 'Ouvrage ou réseau', 0),
    ('ADVERTISEMENT',  'Réclame',        'Procédé de réclame', 0),
    ('SPORT_ZONE',     'Zone sportive',  'Installation ou zone de sport', 0)
ON CONFLICT (code) DO NOTHING;

-- Land-rights roles of actors on a thing (roadmap GLD-016).
INSERT INTO relationship_type (code, label, source_kind, target_kind, inverse_label, description, is_directed) VALUES
    ('THING_HAS_ACTOR_OWNER',                'Objet a propriétaire',     'THING', 'ACTOR', 'Propriétaire de',          'Propriétaire ou copropriétaire', true),
    ('THING_HAS_ACTOR_TENANT',               'Objet a locataire',        'THING', 'ACTOR', 'Locataire de',             'Locataire', true),
    ('THING_HAS_ACTOR_SURFACE_RIGHT_HOLDER', 'Objet a superficiaire',    'THING', 'ACTOR', 'Superficiaire de',         'Titulaire d''un droit de superficie', true),
    ('THING_HAS_ACTOR_FARMER',               'Objet a fermier',          'THING', 'ACTOR', 'Fermier de',               'Fermier (bail à ferme)', true),
    ('THING_HAS_ACTOR_EASEMENT_HOLDER',      'Objet a bénéficiaire de servitude', 'THING', 'ACTOR', 'Bénéficiaire de servitude sur', 'Bénéficiaire d''une servitude', true)
ON CONFLICT (code) DO NOTHING;

-- Thing types become administrable reference data (GLD-040).
ALTER TABLE reference_change DROP CONSTRAINT reference_change_catalogue_valid;
ALTER TABLE reference_change ADD CONSTRAINT reference_change_catalogue_valid CHECK (
    catalogue IN ('case_type', 'relationship_type', 'organization_category', 'document_type', 'thing_type')
);

-- migrate:down

ALTER TABLE reference_change DROP CONSTRAINT reference_change_catalogue_valid;
ALTER TABLE reference_change ADD CONSTRAINT reference_change_catalogue_valid CHECK (
    catalogue IN ('case_type', 'relationship_type', 'organization_category', 'document_type')
);
DELETE FROM relationship_type WHERE code IN (
    'THING_HAS_ACTOR_OWNER', 'THING_HAS_ACTOR_TENANT', 'THING_HAS_ACTOR_SURFACE_RIGHT_HOLDER',
    'THING_HAS_ACTOR_FARMER', 'THING_HAS_ACTOR_EASEMENT_HOLDER');
DROP TABLE IF EXISTS thing_building;
DROP TABLE IF EXISTS thing_parcel;
DROP TRIGGER IF EXISTS trg_thing_set_updated_at ON thing;
DROP FUNCTION IF EXISTS set_thing_updated_at();
DROP TABLE IF EXISTS thing;
DROP TABLE IF EXISTS thing_type;
