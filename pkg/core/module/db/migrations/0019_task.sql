-- migrate:up

-- Goéland POC — case tasks (spec v2 §28, roadmap GLD-026).
--
-- A task is work to do on a case, independent of any workflow: manual today,
-- later created by a circulation, a workflow or an AI proposal (origin). Like
-- a timeline entry it belongs to its case and is not a subject: its audit
-- events are written on the CASE subject. The legacy system has no task
-- entity, so this model is new.
--
-- Lifecycle: OPEN → IN_PROGRESS → DONE, or CANCELLED with a reason; a done or
-- cancelled task may be reopened with a reason. A task is assigned to one
-- internal user or one org unit, or to nobody yet; every (re)assignment is kept
-- in case_task_assignment.

-- Controlled classification, administrable (GLD-040) --------------------------------
CREATE TABLE task_type (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        TEXT UNIQUE NOT NULL,
    label       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    is_active   BOOLEAN NOT NULL DEFAULT true,

    CONSTRAINT task_type_code_not_blank CHECK (length(btrim(code)) > 0)
);

INSERT INTO task_type (code, label, description) VALUES
    ('DOCUMENT_CHECK',  'Contrôle de dossier', 'Vérifier les pièces et la complétude du dossier'),
    ('OPINION_REQUEST', 'Demande de préavis',  'Obtenir le préavis d''un service ou d''une instance'),
    ('SITE_VISIT',      'Visite sur place',    'Se rendre sur le site concerné par l''affaire'),
    ('CALLBACK',        'Rappel',              'Recontacter un intervenant'),
    ('OTHER',           'Autre',               'Tâche sans type particulier')
ON CONFLICT (code) DO NOTHING;

-- Tasks -------------------------------------------------------------------------------
CREATE TABLE case_task (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id              UUID NOT NULL REFERENCES case_file (id),
    task_type_id         UUID NOT NULL REFERENCES task_type (id),
    title                TEXT NOT NULL,
    description          TEXT NOT NULL DEFAULT '',
    status               SMALLINT NOT NULL DEFAULT 1, -- 1=OPEN 2=IN_PROGRESS 3=DONE 4=CANCELLED
    -- 1=MANUAL 2=CIRCULATION 3=WORKFLOW 4=AI; origin_ref names the creating object.
    origin               SMALLINT NOT NULL DEFAULT 1,
    origin_ref           TEXT NOT NULL DEFAULT '',
    assignee_user_id     TEXT REFERENCES app_user (user_id),
    assignee_org_unit_id UUID REFERENCES org_unit (id),
    due_at               TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by           TEXT NOT NULL DEFAULT '',
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by           TEXT NOT NULL DEFAULT '',
    started_at           TIMESTAMPTZ,
    started_by           TEXT NOT NULL DEFAULT '',
    completed_at         TIMESTAMPTZ,
    completed_by         TEXT NOT NULL DEFAULT '',
    completion_note      TEXT NOT NULL DEFAULT '',
    cancelled_at         TIMESTAMPTZ,
    cancelled_by         TEXT NOT NULL DEFAULT '',
    cancellation_reason  TEXT NOT NULL DEFAULT '',

    CONSTRAINT case_task_title_not_blank CHECK (length(btrim(title)) > 0),
    CONSTRAINT case_task_status_valid CHECK (status BETWEEN 1 AND 4),
    CONSTRAINT case_task_origin_valid CHECK (origin BETWEEN 1 AND 4),
    CONSTRAINT case_task_one_assignee CHECK (assignee_user_id IS NULL OR assignee_org_unit_id IS NULL),
    -- Done and cancelled stamps exist exactly in their status; a cancellation has a reason.
    CONSTRAINT case_task_stamps_consistent CHECK (
        (status = 3) = (completed_at IS NOT NULL)
        AND (status = 4) = (cancelled_at IS NOT NULL)
        AND (status = 4) = (length(btrim(cancellation_reason)) > 0)
    )
);

CREATE INDEX idx_case_task_case ON case_task (case_id, status);
CREATE INDEX idx_case_task_user_open ON case_task (assignee_user_id) WHERE status IN (1, 2);
CREATE INDEX idx_case_task_unit_open ON case_task (assignee_org_unit_id) WHERE status IN (1, 2);
CREATE INDEX idx_case_task_due ON case_task (due_at) WHERE status IN (1, 2);

-- migrate:statementbegin
CREATE OR REPLACE FUNCTION set_case_task_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- migrate:statementend

CREATE TRIGGER trg_case_task_set_updated_at
    BEFORE UPDATE ON case_task
    FOR EACH ROW
    EXECUTE FUNCTION set_case_task_updated_at();

-- Assignment history: the current assignment has no ended_at; an unassigned
-- task has no current row. Rows are never deleted.
CREATE TABLE case_task_assignment (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id              UUID NOT NULL REFERENCES case_task (id),
    assignee_user_id     TEXT REFERENCES app_user (user_id),
    assignee_org_unit_id UUID REFERENCES org_unit (id),
    assigned_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    assigned_by          TEXT NOT NULL DEFAULT '',
    reason               TEXT NOT NULL DEFAULT '',
    ended_at             TIMESTAMPTZ,

    CONSTRAINT case_task_assignment_exactly_one CHECK ((assignee_user_id IS NULL) <> (assignee_org_unit_id IS NULL))
);

CREATE UNIQUE INDEX idx_case_task_assignment_current ON case_task_assignment (task_id) WHERE ended_at IS NULL;
CREATE INDEX idx_case_task_assignment_task ON case_task_assignment (task_id, assigned_at);

-- Internal users belong to org units ("my units' tasks"); a typed relationship.
INSERT INTO relationship_type (code, label, source_kind, target_kind, inverse_label, description, is_directed) VALUES
    ('USER_MEMBER_OF_ORG_UNIT', 'Utilisateur membre de l''unité', 'USER', 'ORG_UNIT', 'Membre', 'Appartenance d''un utilisateur interne à une unité', true)
ON CONFLICT (code) DO NOTHING;

-- The type catalogue is administered like the others (GLD-040).
ALTER TABLE reference_change DROP CONSTRAINT reference_change_catalogue_valid;
ALTER TABLE reference_change ADD CONSTRAINT reference_change_catalogue_valid CHECK (
    catalogue IN ('case_type', 'relationship_type', 'organization_category', 'document_type', 'thing_type', 'org_unit_type', 'task_type')
);

-- migrate:down

ALTER TABLE reference_change DROP CONSTRAINT reference_change_catalogue_valid;
ALTER TABLE reference_change ADD CONSTRAINT reference_change_catalogue_valid CHECK (
    catalogue IN ('case_type', 'relationship_type', 'organization_category', 'document_type', 'thing_type', 'org_unit_type')
);

DELETE FROM relationship_type WHERE code = 'USER_MEMBER_OF_ORG_UNIT';

DROP TABLE IF EXISTS case_task_assignment;
DROP TRIGGER IF EXISTS trg_case_task_set_updated_at ON case_task;
DROP FUNCTION IF EXISTS set_case_task_updated_at();
DROP TABLE IF EXISTS case_task;
DROP TABLE IF EXISTS task_type;
