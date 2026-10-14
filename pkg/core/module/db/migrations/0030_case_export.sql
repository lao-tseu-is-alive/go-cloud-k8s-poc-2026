-- migrate:up

-- Goéland POC — case export jobs (spec v2 §51, roadmap GLD-029).
--
-- An export is requested (QUEUED), built in the background by a worker of any
-- replica (RUNNING; claimed with FOR UPDATE SKIP LOCKED), stored as a ZIP in the
-- blob store (DONE, storage_ref) and removed after its time to live (EXPIRED).
-- The job is not a subject: who exported what is recorded as EXPORT in
-- access_audit_event (GLD-033); this table only drives the work and the
-- download. Rows are kept as history, the archive bytes are not.

CREATE TABLE case_export (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id      UUID NOT NULL REFERENCES case_file (id),
    -- the operator id (server-side, never from the request); the only downloader
    requested_by TEXT NOT NULL,
    reason       TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'QUEUED',
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at   TIMESTAMPTZ,
    finished_at  TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ,
    -- builds started, so a job stuck by a crashed worker is retried a bounded number of times
    attempts     SMALLINT NOT NULL DEFAULT 0,
    storage_ref  TEXT NOT NULL DEFAULT '',
    sha256       TEXT NOT NULL DEFAULT '',
    size_bytes   BIGINT NOT NULL DEFAULT 0,
    error        TEXT NOT NULL DEFAULT '',
    request_id   TEXT NOT NULL DEFAULT '',

    CONSTRAINT case_export_requested_by_not_blank CHECK (length(btrim(requested_by)) > 0),
    CONSTRAINT case_export_status_valid CHECK (status IN ('QUEUED', 'RUNNING', 'DONE', 'FAILED', 'EXPIRED')),
    CONSTRAINT case_export_done_has_archive CHECK (status <> 'DONE' OR (storage_ref <> '' AND expires_at IS NOT NULL))
);

-- the worker's queue, the exports of a case, and the archives to expire
CREATE INDEX idx_case_export_queue ON case_export (requested_at) WHERE status IN ('QUEUED', 'RUNNING');
CREATE INDEX idx_case_export_case ON case_export (case_id, requested_at DESC);
CREATE INDEX idx_case_export_expiry ON case_export (expires_at) WHERE status = 'DONE';

-- migrate:down

DROP TABLE IF EXISTS case_export;
