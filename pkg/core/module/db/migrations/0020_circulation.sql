-- migrate:up

-- Goéland POC — case circulations (spec v2 §29, spec v1 §9, roadmap GLD-013).
--
-- A circulation sends a case to several recipients for their answer and is a
-- light orchestration of tasks: each recipient (one internal user or one org
-- unit) gets a task (origin CIRCULATION, origin_ref = circulation id) when its
-- step opens. Recipients sharing a step answer in parallel; the next step opens
-- once the previous one has fully answered (production circulations are ordered
-- in 60% of cases). Every response completes its task and is recorded as a
-- locked RESPONSE timeline entry; the last one completes the circulation.
-- Like tasks, a circulation belongs to its case and is audited on the CASE
-- subject. Overdue is computed from due_at (no EXPIRED status yet, GLD-028).

CREATE TABLE case_circulation (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id             UUID NOT NULL REFERENCES case_file (id),
    title               TEXT NOT NULL,
    message             TEXT NOT NULL DEFAULT '',
    due_at              TIMESTAMPTZ,
    status              SMALLINT NOT NULL DEFAULT 1, -- 1=OPEN 2=COMPLETED 3=CANCELLED
    current_step        INT NOT NULL DEFAULT 1,
    step_count          INT NOT NULL DEFAULT 1,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by          TEXT NOT NULL DEFAULT '',
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at        TIMESTAMPTZ,
    cancelled_at        TIMESTAMPTZ,
    cancelled_by        TEXT NOT NULL DEFAULT '',
    cancellation_reason TEXT NOT NULL DEFAULT '',

    CONSTRAINT case_circulation_title_not_blank CHECK (length(btrim(title)) > 0),
    CONSTRAINT case_circulation_status_valid CHECK (status BETWEEN 1 AND 3),
    CONSTRAINT case_circulation_steps_valid CHECK (current_step BETWEEN 1 AND step_count),
    CONSTRAINT case_circulation_stamps_consistent CHECK (
        (status = 2) = (completed_at IS NOT NULL)
        AND (status = 3) = (cancelled_at IS NOT NULL)
        AND (status = 3) = (length(btrim(cancellation_reason)) > 0)
    )
);

CREATE INDEX idx_case_circulation_case ON case_circulation (case_id, status);
CREATE INDEX idx_case_circulation_due ON case_circulation (due_at) WHERE status = 1;

-- migrate:statementbegin
CREATE OR REPLACE FUNCTION set_case_circulation_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- migrate:statementend

CREATE TRIGGER trg_case_circulation_set_updated_at
    BEFORE UPDATE ON case_circulation
    FOR EACH ROW
    EXECUTE FUNCTION set_case_circulation_updated_at();

-- Recipients: one user or one unit, once per circulation; the task exists once
-- the recipient's step is open; a response is given once.
CREATE TABLE case_circulation_recipient (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    circulation_id       UUID NOT NULL REFERENCES case_circulation (id),
    step                 INT NOT NULL DEFAULT 1,
    assignee_user_id     TEXT REFERENCES app_user (user_id),
    assignee_org_unit_id UUID REFERENCES org_unit (id),
    task_id              UUID UNIQUE REFERENCES case_task (id),
    -- 1=FAVORABLE 2=UNFAVORABLE 3=COMMENT 4=NOT_CONCERNED 5=NEED_MORE_INFO
    response             SMALLINT,
    response_text        TEXT NOT NULL DEFAULT '',
    responded_at         TIMESTAMPTZ,
    responded_by         TEXT NOT NULL DEFAULT '',
    response_entry_id    UUID REFERENCES case_timeline_entry (id),

    CONSTRAINT case_circulation_recipient_exactly_one CHECK ((assignee_user_id IS NULL) <> (assignee_org_unit_id IS NULL)),
    CONSTRAINT case_circulation_recipient_step_positive CHECK (step >= 1),
    CONSTRAINT case_circulation_recipient_response_valid CHECK (response IS NULL OR response BETWEEN 1 AND 5),
    -- A response has its stamps, its task and its timeline entry.
    CONSTRAINT case_circulation_recipient_response_consistent CHECK (
        (response IS NULL) = (responded_at IS NULL)
        AND (response IS NULL OR (task_id IS NOT NULL AND response_entry_id IS NOT NULL))
    )
);

CREATE INDEX idx_case_circulation_recipient_circulation ON case_circulation_recipient (circulation_id, step);
CREATE UNIQUE INDEX idx_case_circulation_recipient_user_once
    ON case_circulation_recipient (circulation_id, assignee_user_id) WHERE assignee_user_id IS NOT NULL;
CREATE UNIQUE INDEX idx_case_circulation_recipient_unit_once
    ON case_circulation_recipient (circulation_id, assignee_org_unit_id) WHERE assignee_org_unit_id IS NOT NULL;

-- The task type of the recipients' tasks.
INSERT INTO task_type (code, label, description) VALUES
    ('CIRCULATION_RESPONSE', 'Réponse à une circulation', 'Donner la réponse attendue d''un destinataire de circulation')
ON CONFLICT (code) DO NOTHING;

-- migrate:down

DELETE FROM task_type WHERE code = 'CIRCULATION_RESPONSE';
DROP TABLE IF EXISTS case_circulation_recipient;
DROP TRIGGER IF EXISTS trg_case_circulation_set_updated_at ON case_circulation;
DROP FUNCTION IF EXISTS set_case_circulation_updated_at();
DROP TABLE IF EXISTS case_circulation;
