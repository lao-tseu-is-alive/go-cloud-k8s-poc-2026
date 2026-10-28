-- migrate:up

-- Goéland POC — retention of closed cases, step A (roadmap GLD-032, v2 §35-36).
--
-- A retention policy says how long a closed case is kept and what happens
-- then: ARCHIVE (transfer to the archives), DESTROY (elimination) or REVIEW
-- (decided case by case). A case type names its default policy. When a case
-- is closed, its retention is computed in the same transaction and kept in
-- subject_retention as a snapshot (the policy's duration and disposition at
-- that moment: a later edit of the policy leaves it unchanged); reopening the
-- case ends that row, closing it again starts a new one. A legal hold blocks
-- any disposal while it lasts. Rows are ended, never deleted.
--
-- The case is the unit: its documents follow it (step B). Nothing here
-- disposes of anything; record_metadata.retention_until / sort_final stay as
-- the V1 preparation fields (v2 §36), no longer written.

CREATE TABLE retention_policy (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code              TEXT UNIQUE NOT NULL,
    label             TEXT NOT NULL DEFAULT '',
    description       TEXT NOT NULL DEFAULT '',
    legal_basis       TEXT NOT NULL DEFAULT '',
    duration_years    SMALLINT NOT NULL,
    final_disposition TEXT NOT NULL,
    is_active         BOOLEAN NOT NULL DEFAULT true,

    CONSTRAINT retention_policy_code_not_blank CHECK (length(btrim(code)) > 0),
    CONSTRAINT retention_policy_duration_range CHECK (duration_years BETWEEN 0 AND 200),
    CONSTRAINT retention_policy_disposition_valid CHECK (final_disposition IN ('ARCHIVE', 'DESTROY', 'REVIEW'))
);

-- Examples only: the real schedule comes from the City archives.
INSERT INTO retention_policy (code, label, description, duration_years, final_disposition) VALUES
    ('EXAMPLE_ARCHIVE_10', 'Exemple — 10 ans puis versement', 'Politique d''exemple : conservée 10 ans après la clôture, puis versée aux Archives', 10, 'ARCHIVE'),
    ('EXAMPLE_DESTROY_5',  'Exemple — 5 ans puis élimination', 'Politique d''exemple : conservée 5 ans après la clôture, puis éliminée', 5, 'DESTROY'),
    ('EXAMPLE_REVIEW_10',  'Exemple — 10 ans puis tri', 'Politique d''exemple : conservée 10 ans après la clôture, puis triée par l''archiviste', 10, 'REVIEW')
ON CONFLICT (code) DO NOTHING;

ALTER TABLE case_type
    ADD COLUMN default_retention_policy_code TEXT REFERENCES retention_policy (code);

CREATE TABLE subject_retention (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id        UUID NOT NULL REFERENCES subject_ref (id),
    policy_code       TEXT NOT NULL REFERENCES retention_policy (code),
    duration_years    SMALLINT NOT NULL,
    final_disposition TEXT NOT NULL,
    starts_on         DATE NOT NULL,
    until_on          DATE NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by        TEXT NOT NULL,
    ended_at          TIMESTAMPTZ,
    ended_by          TEXT,
    end_reason        TEXT NOT NULL DEFAULT '',

    CONSTRAINT subject_retention_disposition_valid CHECK (final_disposition IN ('ARCHIVE', 'DESTROY', 'REVIEW')),
    CONSTRAINT subject_retention_dates CHECK (until_on >= starts_on),
    CONSTRAINT subject_retention_ended CHECK ((ended_at IS NULL) = (ended_by IS NULL))
);

-- One running retention per subject; the due list reads them by deadline.
CREATE UNIQUE INDEX idx_subject_retention_current ON subject_retention (subject_id) WHERE ended_at IS NULL;
CREATE INDEX idx_subject_retention_due ON subject_retention (until_on, subject_id) WHERE ended_at IS NULL;
CREATE INDEX idx_subject_retention_history ON subject_retention (subject_id, created_at DESC);

CREATE TABLE retention_hold (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id     UUID NOT NULL REFERENCES subject_ref (id),
    reason         TEXT NOT NULL,
    placed_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    placed_by      TEXT NOT NULL,
    released_at    TIMESTAMPTZ,
    released_by    TEXT,
    release_reason TEXT NOT NULL DEFAULT '',

    CONSTRAINT retention_hold_reason_not_blank CHECK (length(btrim(reason)) > 0),
    CONSTRAINT retention_hold_released CHECK ((released_at IS NULL) = (released_by IS NULL))
);

-- One active hold per subject.
CREATE UNIQUE INDEX idx_retention_hold_active ON retention_hold (subject_id) WHERE released_at IS NULL;
CREATE INDEX idx_retention_hold_history ON retention_hold (subject_id, placed_at DESC);

-- The policy catalogue is administered like the others (GLD-040).
ALTER TABLE reference_change DROP CONSTRAINT reference_change_catalogue_valid;
ALTER TABLE reference_change ADD CONSTRAINT reference_change_catalogue_valid CHECK (
    catalogue IN ('case_type', 'relationship_type', 'organization_category', 'document_type', 'thing_type', 'org_unit_type', 'task_type', 'retention_policy')
);

-- The records manager (archivist) follows the retention of every case: READ
-- on the non-confidential cases, the due list and the legal holds.
INSERT INTO app_role (code, label, description, scope_kind, scope_level) VALUES
    ('RECORDS_MANAGER', 'Archiviste', 'Suit la conservation des affaires : échéances, gel et sort final (lecture des affaires non confidentielles)', 'CASE', 1)
ON CONFLICT (code) DO NOTHING;

-- migrate:down

DELETE FROM app_user_role WHERE role_code = 'RECORDS_MANAGER';
DELETE FROM app_role WHERE code = 'RECORDS_MANAGER';

DELETE FROM reference_change WHERE catalogue = 'retention_policy';
ALTER TABLE reference_change DROP CONSTRAINT reference_change_catalogue_valid;
ALTER TABLE reference_change ADD CONSTRAINT reference_change_catalogue_valid CHECK (
    catalogue IN ('case_type', 'relationship_type', 'organization_category', 'document_type', 'thing_type', 'org_unit_type', 'task_type')
);

DROP TABLE IF EXISTS retention_hold;
DROP TABLE IF EXISTS subject_retention;
ALTER TABLE case_type DROP COLUMN IF EXISTS default_retention_policy_code;
DROP TABLE IF EXISTS retention_policy;
