-- migrate:up

-- Goéland POC — default access of a case type (roadmap GLD-050; model GLD-017).
--
-- A case type may make its new cases confidential and carry a template of
-- grants. Both are applied once, when a case is created: the confidentiality
-- is a minimum (a creation asking for less gets it), and the template lines
-- are copied as ordinary access_grant rows next to the creator's FULL_CONTROL
-- and the owning unit's MANAGE (the highest level wins for the same grantee).
-- A later change of the template leaves existing cases unchanged: there is no
-- live inheritance. CREATOR_UNITS stands for the units the creator is a direct
-- member of when the case is created. Template changes are reference data
-- changes: a template is replaced as a whole and its before and after states
-- are logged in reference_change, like the rest of the catalogue.

ALTER TABLE case_type
    ADD COLUMN default_confidentiality_level SMALLINT NOT NULL DEFAULT 0,
    ADD CONSTRAINT case_type_default_confidentiality_range CHECK (default_confidentiality_level BETWEEN 0 AND 5);

CREATE TABLE case_type_default_grant (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_type_id        UUID NOT NULL REFERENCES case_type (id),
    grantee_kind        TEXT NOT NULL,
    grantee_user_id     TEXT,
    grantee_subject_id  UUID REFERENCES subject_ref (id),
    level               SMALLINT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by          TEXT NOT NULL,

    CONSTRAINT case_type_default_grant_level_range CHECK (level BETWEEN 1 AND 4),
    CONSTRAINT case_type_default_grant_grantee CHECK (
        (grantee_kind = 'USER' AND grantee_user_id IS NOT NULL AND length(btrim(grantee_user_id)) > 0 AND grantee_subject_id IS NULL)
        OR (grantee_kind IN ('GROUP', 'ORG_UNIT') AND grantee_subject_id IS NOT NULL AND grantee_user_id IS NULL)
        OR (grantee_kind = 'CREATOR_UNITS' AND grantee_user_id IS NULL AND grantee_subject_id IS NULL))
);

-- One line per grantee in a template.
CREATE UNIQUE INDEX idx_case_type_default_grant_grantee
    ON case_type_default_grant (case_type_id, grantee_kind, coalesce(grantee_user_id, ''), coalesce(grantee_subject_id::text, ''));

-- migrate:down

DROP TABLE IF EXISTS case_type_default_grant;
ALTER TABLE case_type DROP CONSTRAINT IF EXISTS case_type_default_confidentiality_range;
ALTER TABLE case_type DROP COLUMN IF EXISTS default_confidentiality_level;
