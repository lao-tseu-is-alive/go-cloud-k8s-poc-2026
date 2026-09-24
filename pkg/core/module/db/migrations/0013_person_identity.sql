-- migrate:up

-- Goéland POC — minimal identity of a PERSON actor (roadmap GLD-039,
-- IMPLEMENTATION_STATUS §3g decision of 2026-09-24).
--
-- A person gets the minimum needed to identify and address them: salutation,
-- last name and first name. No birth date, AVS number or civil-registry data.
-- The service requires a last name for new or edited persons; existing rows
-- without one stay valid (legacy import cleaning, GLD-019).

ALTER TABLE actor
    ADD COLUMN salutation SMALLINT NOT NULL DEFAULT 0, -- 0=unspecified 1=madame 2=monsieur 3=neutral
    ADD COLUMN last_name  TEXT NOT NULL DEFAULT '',
    ADD COLUMN first_name TEXT NOT NULL DEFAULT '';

ALTER TABLE actor
    ADD CONSTRAINT actor_salutation_valid CHECK (salutation BETWEEN 0 AND 3),
    -- Identity columns belong to persons only.
    ADD CONSTRAINT actor_identity_requires_person CHECK (
        actor_kind = 1 OR (salutation = 0 AND last_name = '' AND first_name = '')
    );

-- Search also matches the person's names (a generated column cannot be altered,
-- so it is recreated with its index).
DROP INDEX IF EXISTS idx_actor_search_vector;
ALTER TABLE actor DROP COLUMN search_vector;
ALTER TABLE actor ADD COLUMN search_vector TSVECTOR GENERATED ALWAYS AS (
    to_tsvector('simple', immutable_unaccent(
        coalesce(display_name, '') || ' ' || coalesce(legal_name, '') || ' ' ||
        coalesce(first_name, '') || ' ' || coalesce(last_name, '')))
) STORED;
CREATE INDEX idx_actor_search_vector ON actor USING gin (search_vector);

-- migrate:down

DROP INDEX IF EXISTS idx_actor_search_vector;
ALTER TABLE actor DROP COLUMN search_vector;
ALTER TABLE actor ADD COLUMN search_vector TSVECTOR GENERATED ALWAYS AS (
    to_tsvector('simple', immutable_unaccent(coalesce(display_name, '') || ' ' || coalesce(legal_name, '')))
) STORED;
CREATE INDEX idx_actor_search_vector ON actor USING gin (search_vector);

ALTER TABLE actor
    DROP CONSTRAINT IF EXISTS actor_identity_requires_person,
    DROP CONSTRAINT IF EXISTS actor_salutation_valid,
    DROP COLUMN IF EXISTS first_name,
    DROP COLUMN IF EXISTS last_name,
    DROP COLUMN IF EXISTS salutation;
