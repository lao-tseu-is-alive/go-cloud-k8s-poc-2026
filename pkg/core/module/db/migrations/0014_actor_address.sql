-- migrate:up

-- Goéland POC — actor addresses and branches (roadmap GLD-014, IMPLEMENTATION_STATUS
-- §3g decision "Branches", production acteur_adresse + lien_acteur_adresse).
--
-- An address is its own row, linked to actors M:N by actor_address, which carries
-- the role of the address for that actor (head office, branch, correspondence,
-- billing, residence, other) and whether it is the principal one. Replacing an
-- actor's addresses is non-destructive: old links get ended_at, never deleted.
-- A branch acting as a distinct party is a separate ORGANIZATION linked by
-- ACTOR_BRANCH_OF_ACTOR; a contact person is a PERSON linked by
-- ACTOR_CONTACT_PERSON_OF_ACTOR.

CREATE TABLE address (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    street        TEXT NOT NULL,
    house_number  TEXT NOT NULL DEFAULT '',
    address_line2 TEXT NOT NULL DEFAULT '', -- c/o, building, floor, ...
    postal_code   TEXT NOT NULL,
    locality      TEXT NOT NULL,
    country_code  TEXT NOT NULL DEFAULT 'CH', -- ISO 3166-1 alpha-2
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by    TEXT NOT NULL DEFAULT '',

    CONSTRAINT address_street_not_blank CHECK (length(btrim(street)) > 0),
    CONSTRAINT address_postal_code_not_blank CHECK (length(btrim(postal_code)) > 0),
    CONSTRAINT address_locality_not_blank CHECK (length(btrim(locality)) > 0),
    CONSTRAINT address_country_code_format CHECK (country_code ~ '^[A-Z]{2}$'),
    CONSTRAINT address_swiss_postal_code CHECK (country_code <> 'CH' OR postal_code ~ '^[1-9][0-9]{3}$')
);

CREATE TABLE actor_address (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id     UUID NOT NULL REFERENCES actor (id),
    address_id   UUID NOT NULL REFERENCES address (id),
    address_type SMALLINT NOT NULL, -- mirrors the AddressType proto enum
    is_principal BOOLEAN NOT NULL DEFAULT false,
    label        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by   TEXT NOT NULL DEFAULT '',
    ended_at     TIMESTAMPTZ,
    ended_by     TEXT NOT NULL DEFAULT '',

    CONSTRAINT actor_address_type_valid CHECK (address_type BETWEEN 1 AND 6)
);

CREATE INDEX idx_actor_address_actor ON actor_address (actor_id) WHERE ended_at IS NULL;
CREATE INDEX idx_actor_address_address ON actor_address (address_id);
-- At most one current principal address per actor.
CREATE UNIQUE INDEX idx_actor_address_one_principal
    ON actor_address (actor_id) WHERE is_principal AND ended_at IS NULL;

INSERT INTO relationship_type (code, label, source_kind, target_kind, inverse_label, description, is_directed) VALUES
    ('ACTOR_BRANCH_OF_ACTOR',         'Succursale de',           'ACTOR', 'ACTOR', 'A pour succursale',        'Succursale ou établissement agissant comme partie distincte', true),
    ('ACTOR_CONTACT_PERSON_OF_ACTOR', 'Personne de contact de',  'ACTOR', 'ACTOR', 'A pour personne de contact', 'Personne de contact au sein d''une organisation', true)
ON CONFLICT (code) DO NOTHING;

-- migrate:down

DELETE FROM relationship_type WHERE code IN ('ACTOR_BRANCH_OF_ACTOR', 'ACTOR_CONTACT_PERSON_OF_ACTOR');
DROP TABLE IF EXISTS actor_address;
DROP TABLE IF EXISTS address;
