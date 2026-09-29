-- migrate:up

-- Goéland POC — case timeline (suivis, spec v2 §26-27, roadmap GLD-012).
--
-- A timeline entry is part of its case, not a subject of its own: it has no
-- subject_ref, and its audit events are written on the CASE subject (with the
-- entry id in metadata), so the case audit trail shows the whole history.
--
-- Lifecycle: DRAFT (editable) → VALIDATED (endorsed) | LOCKED (frozen as is) |
-- WITHDRAWN (a draft set aside, kept). Only drafts change; a validated, locked or
-- withdrawn entry is immutable and a correction is a new entry pointing at the
-- entry it corrects (corrects_entry_id). SYSTEM entries are written by the server
-- and born LOCKED. Rows are never deleted.

-- Timeline entries -------------------------------------------------------------------
CREATE TABLE case_timeline_entry (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id           UUID NOT NULL REFERENCES case_file (id),
    -- 1=COMMENT 2=OPINION 3=DECISION 4=REQUEST 5=RESPONSE 6=VALIDATION 7=SYSTEM 8=AI_PROPOSAL
    entry_type        SMALLINT NOT NULL,
    status            SMALLINT NOT NULL DEFAULT 1, -- 1=DRAFT 2=VALIDATED 3=LOCKED 4=WITHDRAWN
    title             TEXT NOT NULL DEFAULT '',
    body              TEXT NOT NULL,
    -- 1=CASE_PARTICIPANTS 2=INTERNAL 3=RESTRICTED; stored, enforced with GLD-017.
    visibility        SMALLINT NOT NULL DEFAULT 1,
    -- Business date of the event (a call yesterday, a letter received on...),
    -- distinct from created_at, the recording time. The timeline is ordered on it.
    occurred_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    corrects_entry_id UUID,
    -- Secondary structured data, e.g. the event behind a SYSTEM entry
    -- ({"event":"CASE_STATUS_CHANGED","from":"OPEN","to":"CLOSED"}) so clients
    -- can render it in their language; body stays the readable fallback.
    metadata          JSONB NOT NULL DEFAULT '{}',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by        TEXT NOT NULL DEFAULT '',
    updated_at        TIMESTAMPTZ,
    updated_by        TEXT NOT NULL DEFAULT '',
    validated_at      TIMESTAMPTZ,
    validated_by      TEXT NOT NULL DEFAULT '',
    locked_at         TIMESTAMPTZ,
    locked_by         TEXT NOT NULL DEFAULT '',
    withdrawn_at      TIMESTAMPTZ,
    withdrawn_by      TEXT NOT NULL DEFAULT '',
    withdrawal_reason TEXT NOT NULL DEFAULT '',

    -- Lets a correction reference an entry of the same case (composite FK below).
    CONSTRAINT case_timeline_entry_id_case_unique UNIQUE (id, case_id),
    CONSTRAINT case_timeline_entry_corrects_same_case
        FOREIGN KEY (corrects_entry_id, case_id) REFERENCES case_timeline_entry (id, case_id),
    CONSTRAINT case_timeline_entry_not_self_correction CHECK (corrects_entry_id <> id),
    CONSTRAINT case_timeline_entry_type_valid CHECK (entry_type BETWEEN 1 AND 8),
    CONSTRAINT case_timeline_entry_status_valid CHECK (status BETWEEN 1 AND 4),
    CONSTRAINT case_timeline_entry_visibility_valid CHECK (visibility BETWEEN 1 AND 3),
    CONSTRAINT case_timeline_entry_body_not_blank CHECK (length(btrim(body)) > 0),
    -- Each terminal status carries exactly its own stamps.
    CONSTRAINT case_timeline_entry_stamps_consistent CHECK (
        (status = 2) = (validated_at IS NOT NULL)
        AND (status = 3) = (locked_at IS NOT NULL)
        AND (status = 4) = (withdrawn_at IS NOT NULL)
        AND (status = 4) = (length(btrim(withdrawal_reason)) > 0)
    ),
    -- SYSTEM entries are server-written facts, frozen from birth.
    CONSTRAINT case_timeline_entry_system_locked CHECK (entry_type <> 7 OR status = 3)
);

CREATE INDEX idx_case_timeline_entry_case
    ON case_timeline_entry (case_id, occurred_at DESC, created_at DESC);

-- At most one live (not withdrawn) correction per corrected entry.
CREATE UNIQUE INDEX idx_case_timeline_entry_one_correction
    ON case_timeline_entry (corrects_entry_id)
    WHERE corrects_entry_id IS NOT NULL AND status <> 4;

-- Only a draft changes, and its identity never does; no entry is ever deleted.
-- migrate:statementbegin
CREATE OR REPLACE FUNCTION guard_case_timeline_entry() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'case_timeline_entry rows are never deleted (entry %)', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    IF OLD.status <> 1 THEN
        RAISE EXCEPTION 'case_timeline_entry % is no longer a draft and is immutable', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    IF NEW.case_id <> OLD.case_id OR NEW.created_at <> OLD.created_at OR NEW.created_by <> OLD.created_by
        OR NEW.corrects_entry_id IS DISTINCT FROM OLD.corrects_entry_id THEN
        RAISE EXCEPTION 'case_timeline_entry % identity is immutable', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- migrate:statementend

CREATE TRIGGER trg_guard_case_timeline_entry
    BEFORE UPDATE OR DELETE ON case_timeline_entry
    FOR EACH ROW
    EXECUTE FUNCTION guard_case_timeline_entry();

-- Documents cited by an entry ----------------------------------------------------------
-- The link targets the logical document (v2 §27); the version current at
-- validation is pinned so a decision keeps pointing at the exact probative
-- version. Removing a link from a draft stamps removed_at (kept as history).
ALTER TABLE document_version
    ADD CONSTRAINT document_version_id_document_unique UNIQUE (id, document_id);

CREATE TABLE timeline_document_link (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timeline_entry_id   UUID NOT NULL REFERENCES case_timeline_entry (id),
    document_id         UUID NOT NULL REFERENCES document (id),
    document_version_id UUID,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by          TEXT NOT NULL DEFAULT '',
    removed_at          TIMESTAMPTZ,
    removed_by          TEXT NOT NULL DEFAULT '',

    -- A pinned version must belong to the linked document.
    CONSTRAINT timeline_document_link_version_of_document
        FOREIGN KEY (document_version_id, document_id) REFERENCES document_version (id, document_id)
);

CREATE UNIQUE INDEX idx_timeline_document_link_live_unique
    ON timeline_document_link (timeline_entry_id, document_id)
    WHERE removed_at IS NULL;
CREATE INDEX idx_timeline_document_link_document ON timeline_document_link (document_id);

-- Links change only while their entry is a draft (pinning happens just before
-- validation), only in their removal stamps and pinned version, and are never deleted.
-- migrate:statementbegin
CREATE OR REPLACE FUNCTION guard_timeline_document_link() RETURNS trigger AS $$
DECLARE
    entry_status SMALLINT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'timeline_document_link rows are never deleted (link %)', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    SELECT status INTO entry_status FROM case_timeline_entry WHERE id = NEW.timeline_entry_id;
    IF entry_status <> 1 THEN
        RAISE EXCEPTION 'the documents of timeline entry % are immutable', NEW.timeline_entry_id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    IF TG_OP = 'UPDATE' AND (NEW.timeline_entry_id <> OLD.timeline_entry_id OR NEW.document_id <> OLD.document_id
        OR NEW.created_at <> OLD.created_at OR NEW.created_by <> OLD.created_by
        OR OLD.removed_at IS NOT NULL) THEN
        RAISE EXCEPTION 'timeline_document_link % identity is immutable', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- migrate:statementend

CREATE TRIGGER trg_guard_timeline_document_link
    BEFORE INSERT OR UPDATE OR DELETE ON timeline_document_link
    FOR EACH ROW
    EXECUTE FUNCTION guard_timeline_document_link();

-- migrate:down

DROP TRIGGER IF EXISTS trg_guard_timeline_document_link ON timeline_document_link;
DROP FUNCTION IF EXISTS guard_timeline_document_link();
DROP TABLE IF EXISTS timeline_document_link;
ALTER TABLE document_version DROP CONSTRAINT IF EXISTS document_version_id_document_unique;
DROP TRIGGER IF EXISTS trg_guard_case_timeline_entry ON case_timeline_entry;
DROP FUNCTION IF EXISTS guard_case_timeline_entry();
DROP TABLE IF EXISTS case_timeline_entry;
