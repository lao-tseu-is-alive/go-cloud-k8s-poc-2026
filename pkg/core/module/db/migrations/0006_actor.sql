-- migrate:up

-- Goéland POC — Actor component (external persons & organizations, spec §6.4).
--
-- An actor is a first-class subject: actor.id IS a subject_ref.id of kind ACTOR
-- (the composite FK pins the kind). Governance lives in record_metadata; typed links
-- to cases/documents/things live in subject_relationship (CASE_HAS_ACTOR_*, ...), never
-- as columns here — so roles stay in relationship_type and grow slice by slice.
--
-- Model informed by the production Goéland (MSSQL) Acteur domain:
--   * Acteur base   -> actor (actor_kind from IsPhysique; is_active from IsActive;
--                      display_name from Name; name_for_search from NameForSearch;
--                      publication_code from CodePublication)
--   * ActMoral      -> ORGANIZATION specialization columns (legal_name = RaisonSociale,
--                      organization_category_id = IdCategory, org_complement = Complement)
--   * ActPhysFromCH -> PERSON specialization carries NO personal data: only an
--                      is_ch_register flag + opaque ch_register_ref (real identity stays
--                      in the source system).
--   * ActeurComplement -> actor_contact (typed contact channels + business identifiers).

-- Controlled classification of organizations (DicoActMoralCategory) --------------
CREATE TABLE organization_category (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code      TEXT UNIQUE NOT NULL,
    label     TEXT NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT true,

    CONSTRAINT organization_category_code_not_blank CHECK (length(btrim(code)) > 0)
);

-- The actor entity (base + flattened person/organization specialization) ----------
CREATE TABLE actor (
    id                       UUID PRIMARY KEY,
    kind                     TEXT NOT NULL DEFAULT 'ACTOR',
    actor_kind               SMALLINT NOT NULL, -- 1=PERSON 2=ORGANIZATION
    display_name             TEXT NOT NULL,
    name_for_search          TEXT NOT NULL DEFAULT '',
    is_active                BOOLEAN NOT NULL DEFAULT true, -- business deactivation (Acteur.IsActive), distinct from soft-delete
    publication_code         INT NOT NULL DEFAULT 0,        -- Acteur.CodePublication (opaque passthrough)

    -- ORGANIZATION specialization (ActMoral)
    legal_name               TEXT NOT NULL DEFAULT '',      -- RaisonSociale
    organization_category_id UUID REFERENCES organization_category (id),
    org_complement           TEXT NOT NULL DEFAULT '',

    -- PERSON specialization (no PII): CH population-register link only
    is_ch_register           BOOLEAN NOT NULL DEFAULT false, -- Acteur.IsFromCH
    ch_register_ref          TEXT NOT NULL DEFAULT '',       -- opaque external key, never civil data

    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by               TEXT NOT NULL DEFAULT '',
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Accent-insensitive full-text search over the searchable names (reuses the
    -- immutable_unaccent wrapper from migration 0005): "chateau" finds "château".
    search_vector            TSVECTOR GENERATED ALWAYS AS (
        to_tsvector('simple', immutable_unaccent(coalesce(display_name, '') || ' ' || coalesce(legal_name, '')))
    ) STORED,

    -- actor.id must reference a subject_ref row whose kind is ACTOR.
    CONSTRAINT actor_subject_fkey FOREIGN KEY (id, kind) REFERENCES subject_ref (id, kind),
    CONSTRAINT actor_kind_is_actor CHECK (kind = 'ACTOR'),
    CONSTRAINT actor_actor_kind_valid CHECK (actor_kind BETWEEN 1 AND 2),
    CONSTRAINT actor_display_name_not_blank CHECK (length(btrim(display_name)) > 0),
    CONSTRAINT actor_publication_code_non_negative CHECK (publication_code >= 0),
    -- ORGANIZATION-only columns must be empty for a PERSON.
    CONSTRAINT actor_org_fields_require_org CHECK (
        actor_kind = 2 OR (legal_name = '' AND organization_category_id IS NULL AND org_complement = '')
    ),
    -- PERSON-only columns must be empty for an ORGANIZATION.
    CONSTRAINT actor_person_fields_require_person CHECK (
        actor_kind = 1 OR (is_ch_register = false AND ch_register_ref = '')
    )
);

CREATE INDEX idx_actor_kind ON actor (actor_kind);
CREATE INDEX idx_actor_category ON actor (organization_category_id);
CREATE INDEX idx_actor_active ON actor (is_active);
CREATE INDEX idx_actor_search_vector ON actor USING gin (search_vector);

-- Typed contact channels + business identifiers (ActeurComplement) ----------------
CREATE TABLE actor_contact (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id     UUID NOT NULL REFERENCES actor (id) ON DELETE CASCADE,
    contact_type SMALLINT NOT NULL, -- mirrors the ContactType proto enum
    value        TEXT NOT NULL,
    is_primary   BOOLEAN NOT NULL DEFAULT false,
    label        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT actor_contact_type_positive CHECK (contact_type > 0),
    CONSTRAINT actor_contact_value_not_blank CHECK (length(btrim(value)) > 0)
);

CREATE INDEX idx_actor_contact_actor ON actor_contact (actor_id);
-- Business identifiers (IDE / VAT / ABACUS) are looked up by value, so keep it indexed.
CREATE INDEX idx_actor_contact_value ON actor_contact (contact_type, value);

-- Keep updated_at correct even for writes outside the service code.
-- migrate:statementbegin
CREATE OR REPLACE FUNCTION set_actor_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- migrate:statementend

CREATE TRIGGER trg_actor_set_updated_at
    BEFORE UPDATE ON actor
    FOR EACH ROW
    EXECUTE FUNCTION set_actor_updated_at();

-- Seed organization categories (real production DicoActMoralCategory vocabulary) ---
INSERT INTO organization_category (code, label) VALUES
    ('COMMERCE',                'Commerces'),
    ('DIVERS',                  'Divers'),
    ('METIER_BATIMENT',         'Métiers du bâtiment'),
    ('BUREAU_ARCHITECTE',       'Bureau d''architecte'),
    ('ASSOCIATION_UNION',       'Association/Union'),
    ('SOCIETE_IMMOBILIERE',     'Société immobilière'),
    ('CAFE_RESTO_HOTEL',        'Café, Restaurant, Hôtel'),
    ('COMMISSION_CC',           'Commission CC'),
    ('ADMINISTRATION',          'Administration'),
    ('PPE',                     'PPE'),
    ('COMMUNAUTE_HEREDITAIRE',  'Communauté héréditaire'),
    ('BUREAU_INGENIEUR',        'Bureau d''ingénieur'),
    ('PUBLICITE_GRAPHISME',     'Publicité, graphisme'),
    ('AVOCAT_JURISTE',          'Avocat, juriste'),
    ('SANTE',                   'Santé'),
    ('FONDATION',               'Fondation'),
    ('CLUB_SPORTIF',            'Club sportif'),
    ('INFORMATIQUE_TELECOM',    'Informatique et télécommunications'),
    ('ENTREPRISE_TP',           'Entreprise de travaux publics'),
    ('GERANCE_IMMOBILIERE',     'Gérance immobilière'),
    ('PAYSAGISTE_HORTICULTEUR', 'Paysagiste et horticulteur'),
    ('FIDUCIAIRE',              'Fiduciaire'),
    ('GARAGE',                  'Garage'),
    ('ENSEIGNEMENT',            'Enseignement'),
    ('BANQUE_ASSURANCE',        'Banque, Assurance'),
    ('FEDERATION',              'Fédération'),
    ('COPROPRIETE',             'Copropriété'),
    ('NOTAIRE',                 'Notaire'),
    ('CAISSE_PENSION',          'Caisse de pension'),
    ('ACCUEIL_FAMILIAL',        'Accueil milieu familial'),
    ('GEOMETRE',                'Géomètre'),
    ('SOCIETE_DEVELOPPEMENT',   'Société de développement'),
    ('GROUPE_CC',               'Groupe CC')
ON CONFLICT (code) DO NOTHING;

-- migrate:down

DROP TRIGGER IF EXISTS trg_actor_set_updated_at ON actor;
DROP FUNCTION IF EXISTS set_actor_updated_at();
DROP TABLE IF EXISTS actor_contact;
DROP TABLE IF EXISTS actor;
DROP TABLE IF EXISTS organization_category;
