-- migrate:up

-- Goéland POC — access log of every subject (roadmap GLD-089, extends GLD-033).
--
-- Decided 2026-11-26: like the legacy AffaireLogAcces / DocumentLogAcces, every
-- detail read of a subject and every download of a document is logged, allowed
-- or refused, whatever the confidentiality, and so is a change refused for lack
-- of level (WRITE_DENIED); anyone who may read a subject sees its log. READ
-- replaces READ_SENSITIVE (kept for the rows already written; the
-- confidentiality_level column still tells a confidential read). The
-- application records a repeated access (same reader, subject, action, outcome
-- and version) once per 15 minutes; the index on (subject_id, occurred_at)
-- serves that check. The log stays append-only.

ALTER TABLE access_audit_event DROP CONSTRAINT access_audit_event_action_valid;
ALTER TABLE access_audit_event ADD CONSTRAINT access_audit_event_action_valid
    CHECK (action IN ('READ', 'READ_SENSITIVE', 'DOWNLOAD', 'EXPORT', 'WRITE_DENIED'));

-- migrate:down

-- The rows written since cannot be removed (append-only): the former rule
-- applies to new rows only.
ALTER TABLE access_audit_event DROP CONSTRAINT access_audit_event_action_valid;
ALTER TABLE access_audit_event ADD CONSTRAINT access_audit_event_action_valid
    CHECK (action IN ('READ_SENSITIVE', 'DOWNLOAD', 'EXPORT')) NOT VALID;
