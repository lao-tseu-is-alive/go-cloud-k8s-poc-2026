-- migrate:up

-- Goéland POC — sensitive read audit (spec v2 §34, roadmap GLD-033).
--
-- Mutations are audited in audit_event, in their own transaction. This log is
-- distinct: it records who read or downloaded a confidential subject
-- (confidentiality_level >= 2), and who tried to and was refused, so that
-- "who saw this case" has an answer. Ordinary reads are not logged (the legacy
-- AffaireLogAcces logged every case opened: 18.7M rows). The application
-- writes the row before answering and fails the read if it cannot (no trace,
-- no read). Like audit_event, the log is append-only and kept without limit in
-- the POC; its legal retention period is an open production item.

CREATE TABLE access_audit_event (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    occurred_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- the operator id (server-side, never from the request)
    user_id               TEXT NOT NULL,
    subject_id            UUID NOT NULL REFERENCES subject_ref (id),
    subject_kind          TEXT NOT NULL REFERENCES subject_kind (code),
    action                TEXT NOT NULL,
    outcome               TEXT NOT NULL,
    -- the caller's effective level and the rule that produced it, at that moment
    access_level          SMALLINT NOT NULL,
    access_source         TEXT NOT NULL,
    confidentiality_level SMALLINT NOT NULL,
    -- the version whose bytes were downloaded (DOWNLOAD only)
    document_version_id   UUID REFERENCES document_version (id),
    -- reserved: a stated purpose ("break the glass") and a correlation id
    purpose               TEXT,
    correlation_id        UUID,
    request_id            TEXT NOT NULL DEFAULT '',

    CONSTRAINT access_audit_event_user_not_blank CHECK (length(btrim(user_id)) > 0),
    CONSTRAINT access_audit_event_action_valid CHECK (action IN ('READ_SENSITIVE', 'DOWNLOAD', 'EXPORT')),
    CONSTRAINT access_audit_event_outcome_valid CHECK (outcome IN ('GRANTED', 'DENIED')),
    CONSTRAINT access_audit_event_level_range CHECK (access_level BETWEEN 0 AND 4),
    CONSTRAINT access_audit_event_source_valid CHECK (access_source IN ('PERSONAL', 'GROUP', 'ORG_UNIT', 'ROLE', 'BASELINE')),
    CONSTRAINT access_audit_event_version_on_download CHECK (document_version_id IS NULL OR action = 'DOWNLOAD')
);

-- a subject's log, a user's log and the whole log, newest first
CREATE INDEX idx_access_audit_event_subject ON access_audit_event (subject_id, occurred_at DESC, id DESC);
CREATE INDEX idx_access_audit_event_user ON access_audit_event (user_id, occurred_at DESC, id DESC);
CREATE INDEX idx_access_audit_event_occurred ON access_audit_event (occurred_at DESC, id DESC);

CREATE TRIGGER trg_access_audit_event_append_only
    BEFORE UPDATE OR DELETE ON access_audit_event
    FOR EACH ROW
    EXECUTE FUNCTION refuse_append_only_change();

CREATE TRIGGER trg_access_audit_event_no_truncate
    BEFORE TRUNCATE ON access_audit_event
    FOR EACH STATEMENT
    EXECUTE FUNCTION refuse_append_only_change();

-- migrate:down

DROP TABLE IF EXISTS access_audit_event;
