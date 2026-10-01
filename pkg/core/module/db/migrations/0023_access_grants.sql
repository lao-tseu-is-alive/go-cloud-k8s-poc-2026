-- migrate:up

-- Goéland POC — grants and groups (spec v2 §32, roadmap GLD-048; model GLD-017).
--
-- Every subject (case, document, actor, thing, org unit, group) may carry
-- grants: a level READ (1) < CONTRIBUTE (2) < MANAGE (3) < FULL_CONTROL (4)
-- given to a USER, a GROUP or an ORG_UNIT (a unit grant covers its sub-units).
-- The effective level of a user is the most specific one: personal grant, then
-- the highest grant of the user's groups, then the grant of the nearest unit,
-- then a kind-wide application role, then the baseline (READ unless the subject
-- is confidential, confidentiality_level >= 2). There is no deny level: a
-- restriction is a more specific, lower grant. The legacy grants are
-- overwritten in place; here a change or a revocation keeps the row as history
-- and is audited on the subject.

-- Groups: named sets of internal users, for cross-unit audiences -------------------------
INSERT INTO subject_kind (code) VALUES ('GROUP') ON CONFLICT (code) DO NOTHING;

CREATE TABLE security_group (
    id           UUID PRIMARY KEY,
    kind         TEXT NOT NULL DEFAULT 'GROUP',
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    archived_at  TIMESTAMPTZ,
    archived_by  TEXT NOT NULL DEFAULT '',

    CONSTRAINT security_group_subject_fkey FOREIGN KEY (id, kind) REFERENCES subject_ref (id, kind),
    CONSTRAINT security_group_kind_is_group CHECK (kind = 'GROUP'),
    CONSTRAINT security_group_name_not_blank CHECK (length(btrim(name)) > 0)
);

-- Live groups never share a name (accent- and case-insensitively).
CREATE UNIQUE INDEX idx_security_group_live_name
    ON security_group (immutable_unaccent(lower(name))) WHERE archived_at IS NULL;

-- Membership is a typed relationship, ended (kept as history) on removal.
INSERT INTO relationship_type (code, label, source_kind, target_kind, inverse_label, description, is_directed) VALUES
    ('USER_MEMBER_OF_GROUP', 'Utilisateur membre du groupe', 'USER', 'GROUP', 'Membre', 'Appartenance d''un utilisateur interne à un groupe de sécurité', true)
ON CONFLICT (code) DO NOTHING;

-- Kind-wide application roles ------------------------------------------------------------
-- A role may give a level on every subject of one kind (the legacy ActeurManager,
-- ParcelleManager...); ADMIN gives FULL_CONTROL on every kind. Neither applies to
-- a confidential subject.
ALTER TABLE app_role
    ADD COLUMN scope_kind  TEXT REFERENCES subject_kind (code),
    ADD COLUMN scope_level SMALLINT,
    ADD CONSTRAINT app_role_scope_complete CHECK (
        (scope_kind IS NULL AND scope_level IS NULL) OR (scope_kind IS NOT NULL AND scope_level BETWEEN 1 AND 4));

INSERT INTO app_role (code, label, description, scope_kind, scope_level) VALUES
    ('ACTOR_MANAGER', 'Gestionnaire des acteurs', 'Gère tous les acteurs non confidentiels (droit Gérer)', 'ACTOR', 3),
    ('THING_MANAGER', 'Gestionnaire des objets', 'Gère tous les objets non confidentiels (droit Gérer)', 'THING', 3)
ON CONFLICT (code) DO NOTHING;

-- Grants -----------------------------------------------------------------------------------
CREATE TABLE access_grant (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id          UUID NOT NULL REFERENCES subject_ref (id),
    -- USER grants name the operator id (app_user.user_id, not a FK: the creator
    -- of a subject is granted even before its first recorded request); GROUP and
    -- ORG_UNIT grants name the group or unit subject.
    grantee_kind        TEXT NOT NULL,
    grantee_user_id     TEXT,
    grantee_subject_id  UUID REFERENCES subject_ref (id),
    level               SMALLINT NOT NULL,
    granted_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    granted_by          TEXT NOT NULL,
    grant_reason        TEXT NOT NULL DEFAULT '',
    revoked_at          TIMESTAMPTZ,
    revoked_by          TEXT,
    revoke_reason       TEXT NOT NULL DEFAULT '',

    CONSTRAINT access_grant_level_range CHECK (level BETWEEN 1 AND 4),
    CONSTRAINT access_grant_grantee CHECK (
        (grantee_kind = 'USER' AND grantee_user_id IS NOT NULL AND length(btrim(grantee_user_id)) > 0 AND grantee_subject_id IS NULL)
        OR (grantee_kind IN ('GROUP', 'ORG_UNIT') AND grantee_subject_id IS NOT NULL AND grantee_user_id IS NULL)),
    CONSTRAINT access_grant_revocation_complete CHECK (
        (revoked_at IS NULL AND revoked_by IS NULL) OR (revoked_at IS NOT NULL AND revoked_by IS NOT NULL))
);

-- One current grant per (subject, grantee); history rows are revoked.
CREATE UNIQUE INDEX idx_access_grant_current_user
    ON access_grant (subject_id, grantee_user_id) WHERE revoked_at IS NULL AND grantee_kind = 'USER';
CREATE UNIQUE INDEX idx_access_grant_current_subject
    ON access_grant (subject_id, grantee_subject_id) WHERE revoked_at IS NULL AND grantee_kind <> 'USER';
CREATE INDEX idx_access_grant_subject ON access_grant (subject_id) WHERE revoked_at IS NULL;
CREATE INDEX idx_access_grant_grantee_user ON access_grant (grantee_user_id) WHERE revoked_at IS NULL;
CREATE INDEX idx_access_grant_grantee_subject ON access_grant (grantee_subject_id) WHERE revoked_at IS NULL;

-- Existing subjects: their creator gets FULL_CONTROL and their owning live unit
-- MANAGE, as every new subject does from now on.
INSERT INTO access_grant (subject_id, grantee_kind, grantee_user_id, level, granted_by, grant_reason)
SELECT rm.subject_id, 'USER', rm.created_by, 4, 'system:migration', 'creator of the subject (GLD-048)'
FROM record_metadata rm
WHERE length(btrim(rm.created_by)) > 0 AND rm.created_by NOT LIKE 'system%';

INSERT INTO access_grant (subject_id, grantee_kind, grantee_subject_id, level, granted_by, grant_reason)
SELECT rm.subject_id, 'ORG_UNIT', rm.owner_org_id, 3, 'system:migration', 'owning unit of the subject (GLD-048)'
FROM record_metadata rm
JOIN org_unit ou ON ou.id = rm.owner_org_id AND ou.dissolved_at IS NULL;

-- migrate:down

DROP TABLE IF EXISTS access_grant;
DELETE FROM app_role WHERE code IN ('ACTOR_MANAGER', 'THING_MANAGER');
ALTER TABLE app_role DROP CONSTRAINT IF EXISTS app_role_scope_complete;
ALTER TABLE app_role DROP COLUMN IF EXISTS scope_level, DROP COLUMN IF EXISTS scope_kind;
DELETE FROM relationship_type WHERE code = 'USER_MEMBER_OF_GROUP';
DROP TABLE IF EXISTS security_group;
DELETE FROM subject_kind WHERE code = 'GROUP';
