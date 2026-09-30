-- migrate:up

-- Goéland POC — application roles stored in Goéland (spec v2 §32, roadmap GLD-047).
--
-- The token only authenticates: what an internal user may administer is decided,
-- configured and stored here. The legacy system uses about 120 "security groups"
-- as application roles (GoelandManager, AffaireManager, ActeurManager, ...); the
-- POC starts with ADMIN, the role behind the goeland:admin scope, and gains
-- kind-wide roles with the grants of GLD-048.
--
-- An assignment is never deleted: a revocation stamps revoked_* and keeps the row
-- as history. Every grant and revocation is audited on the user's USER subject.

CREATE TABLE app_role (
    code        TEXT PRIMARY KEY,
    label       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    is_active   BOOLEAN NOT NULL DEFAULT true,

    CONSTRAINT app_role_code_format CHECK (code ~ '^[A-Z][A-Z0-9_]*$')
);

INSERT INTO app_role (code, label, description) VALUES
    ('ADMIN', 'Administrateur', 'Administre Goéland : référentiels, unités organisationnelles et rôles des utilisateurs')
ON CONFLICT (code) DO NOTHING;

CREATE TABLE app_user_role (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       TEXT NOT NULL REFERENCES app_user (user_id),
    role_code     TEXT NOT NULL REFERENCES app_role (code),
    granted_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    granted_by    TEXT NOT NULL,
    grant_reason  TEXT NOT NULL DEFAULT '',
    revoked_at    TIMESTAMPTZ,
    revoked_by    TEXT,
    revoke_reason TEXT NOT NULL DEFAULT '',

    CONSTRAINT app_user_role_granted_by_not_blank CHECK (length(btrim(granted_by)) > 0),
    CONSTRAINT app_user_role_revocation_complete CHECK (
        (revoked_at IS NULL AND revoked_by IS NULL) OR (revoked_at IS NOT NULL AND revoked_by IS NOT NULL))
);

-- At most one current assignment of a role to a user; revoked rows are history.
CREATE UNIQUE INDEX idx_app_user_role_current
    ON app_user_role (user_id, role_code) WHERE revoked_at IS NULL;
CREATE INDEX idx_app_user_role_role ON app_user_role (role_code) WHERE revoked_at IS NULL;

-- Continuity: whoever the auth server flagged as administrator at their last
-- request keeps the role, now stored and revocable here.
INSERT INTO app_user_role (user_id, role_code, granted_by, grant_reason)
SELECT user_id, 'ADMIN', 'system:migration', 'administrator flag of the auth server, migrated (GLD-047)'
FROM app_user
WHERE is_admin;

-- The flag is now derived from the ADMIN role.
ALTER TABLE app_user DROP COLUMN is_admin;

-- migrate:down

ALTER TABLE app_user ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT false;
UPDATE app_user u SET is_admin = true
WHERE EXISTS (SELECT 1 FROM app_user_role r WHERE r.user_id = u.user_id AND r.role_code = 'ADMIN' AND r.revoked_at IS NULL);
DROP TABLE IF EXISTS app_user_role;
DROP TABLE IF EXISTS app_role;
