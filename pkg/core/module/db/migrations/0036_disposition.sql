-- migrate:up

-- Goéland POC — disposition bordereaux, step B1 (roadmap GLD-032, v2 §35-36).
--
-- A records manager gathers due cases (retention ended, no legal hold) of one
-- final disposition in a bordereau. Each case is then approved or rejected by
-- a holder of MANAGE on its owning unit, or on the case when it has none; the
-- preparer never approves. Executing the bordereau handles the approved cases
-- only: ARCHIVE queues a complete transfer package (a case_export of purpose
-- TRANSFER, built without read filter, downloadable by records managers) and
-- freezes the case (record_metadata.is_locked, retention ARCHIVED); DESTROY is
-- step B2. The bordereau keeps the proof: the SHA-256 of its manifest (cases,
-- decisions, approvers), computed at execution. Rows are never deleted.

CREATE TABLE disposition_bordereau (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    final_disposition TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'OPEN',
    reason            TEXT NOT NULL,
    prepared_by       TEXT NOT NULL,
    prepared_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at         TIMESTAMPTZ,
    closed_by         TEXT,
    close_reason      TEXT NOT NULL DEFAULT '',
    proof_sha256      TEXT NOT NULL DEFAULT '',
    proof_manifest    JSONB,

    CONSTRAINT disposition_bordereau_disposition_valid CHECK (final_disposition IN ('ARCHIVE', 'DESTROY')),
    CONSTRAINT disposition_bordereau_status_valid CHECK (status IN ('OPEN', 'EXECUTED', 'CANCELLED')),
    CONSTRAINT disposition_bordereau_reason_not_blank CHECK (length(btrim(reason)) > 0),
    CONSTRAINT disposition_bordereau_closed CHECK ((status = 'OPEN') = (closed_at IS NULL)),
    CONSTRAINT disposition_bordereau_proof CHECK (status <> 'EXECUTED' OR (proof_sha256 <> '' AND proof_manifest IS NOT NULL))
);

CREATE INDEX idx_disposition_bordereau_prepared ON disposition_bordereau (prepared_at DESC);

CREATE TABLE disposition_item (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bordereau_id    UUID NOT NULL REFERENCES disposition_bordereau (id),
    case_id         UUID NOT NULL REFERENCES case_file (id),
    retention_id    UUID NOT NULL REFERENCES subject_retention (id),
    -- who approves: the owning unit (MANAGE on it) or, without one, the case itself
    approver_subject_id UUID NOT NULL REFERENCES subject_ref (id),
    decision        TEXT NOT NULL DEFAULT 'PENDING',
    decided_at      TIMESTAMPTZ,
    decided_by      TEXT,
    decision_reason TEXT NOT NULL DEFAULT '',
    -- the transfer package of an executed ARCHIVE item
    export_id       UUID REFERENCES case_export (id),

    CONSTRAINT disposition_item_decision_valid CHECK (decision IN ('PENDING', 'APPROVED', 'REJECTED')),
    CONSTRAINT disposition_item_decided CHECK ((decision = 'PENDING') = (decided_at IS NULL)),
    CONSTRAINT disposition_item_once UNIQUE (bordereau_id, case_id)
);

-- the items of a bordereau, the open items of a case, and the approvals waiting on a subject
CREATE INDEX idx_disposition_item_bordereau ON disposition_item (bordereau_id);
CREATE INDEX idx_disposition_item_case ON disposition_item (case_id);
CREATE INDEX idx_disposition_item_pending ON disposition_item (approver_subject_id) WHERE decision = 'PENDING';

-- The outcome of a retention: ARCHIVED (transferred, frozen) or DISPOSED (step B2).
ALTER TABLE subject_retention
    ADD COLUMN disposition_status TEXT,
    ADD COLUMN disposed_at        TIMESTAMPTZ,
    ADD COLUMN bordereau_id       UUID REFERENCES disposition_bordereau (id),
    ADD CONSTRAINT subject_retention_disposition_status_valid CHECK (disposition_status IN ('ARCHIVED', 'DISPOSED')),
    ADD CONSTRAINT subject_retention_disposed CHECK ((disposition_status IS NULL) = (disposed_at IS NULL));

-- A transfer package is built by the system without read filter and kept
-- until it is transferred; an ordinary export is the requester's view.
ALTER TABLE case_export
    ADD COLUMN purpose TEXT NOT NULL DEFAULT 'COPY',
    ADD CONSTRAINT case_export_purpose_valid CHECK (purpose IN ('COPY', 'TRANSFER'));

-- migrate:down

ALTER TABLE case_export DROP COLUMN IF EXISTS purpose;
ALTER TABLE subject_retention
    DROP COLUMN IF EXISTS bordereau_id,
    DROP COLUMN IF EXISTS disposed_at,
    DROP COLUMN IF EXISTS disposition_status;
DROP TABLE IF EXISTS disposition_item;
DROP TABLE IF EXISTS disposition_bordereau;
