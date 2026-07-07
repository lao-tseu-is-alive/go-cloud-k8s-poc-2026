-- migrate:up

-- Goéland POC — transversal core schema.
--
-- This migration installs the elements common to every business subject:
--   subject_kind    : controlled list of subject kinds (CASE, DOCUMENT, THING, ACTOR, USER, ORG_UNIT).
--   subject_ref     : canonical identity of any subject (one row per Case/Document/Thing/Actor).
--   record_metadata : 1:1 governance record (ownership, confidentiality, soft-delete, locking).
--   audit_event     : append-only probative history written on every mutation.
--
-- Design rules honoured (spec §17):
--   * explicit typed columns for critical fields, JSONB only for secondary extension data;
--   * never physically delete domain records (deleted_at / deleted_by);
--   * every mutation writes an audit event (enforced in the application layer).

-- Extensions -----------------------------------------------------------------
-- pgcrypto provides gen_random_uuid(); PostGIS is enabled up-front so the future
-- THING component (parcelles, bâtiments) can add geometry columns without a
-- disruptive migration. PostGIS must be available on the server (postgis image).
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;
CREATE EXTENSION IF NOT EXISTS postgis;

-- Controlled list of subject kinds -------------------------------------------
CREATE TABLE subject_kind (
    code TEXT PRIMARY KEY
);

-- Canonical identity of every business subject -------------------------------
CREATE TABLE subject_ref (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind          TEXT NOT NULL REFERENCES subject_kind (code),
    display_label TEXT NOT NULL,
    canonical_url TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Allows composite foreign keys that also pin the kind (e.g. document.id must be a DOCUMENT).
    CONSTRAINT subject_ref_id_kind_unique UNIQUE (id, kind),
    CONSTRAINT subject_ref_display_label_not_blank CHECK (length(btrim(display_label)) > 0)
);

CREATE INDEX idx_subject_ref_kind ON subject_ref (kind);
CREATE INDEX idx_subject_ref_display_label
    ON subject_ref USING gin (to_tsvector('simple', display_label));

-- Transversal governance metadata (1:1 with subject_ref) ---------------------
CREATE TABLE record_metadata (
    subject_id            UUID PRIMARY KEY REFERENCES subject_ref (id),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by            TEXT NOT NULL DEFAULT '',
    updated_at            TIMESTAMPTZ,
    updated_by            TEXT NOT NULL DEFAULT '',
    deleted_at            TIMESTAMPTZ,
    deleted_by            TEXT NOT NULL DEFAULT '',
    owner_user_id         TEXT NOT NULL DEFAULT '',
    owner_org_id          TEXT NOT NULL DEFAULT '',
    confidentiality_level INT NOT NULL DEFAULT 0,
    version               INT NOT NULL DEFAULT 1,
    is_locked             BOOLEAN NOT NULL DEFAULT false,
    locked_at             TIMESTAMPTZ,
    locked_by             TEXT NOT NULL DEFAULT '',
    retention_until       TEXT NOT NULL DEFAULT '',
    sort_final            TEXT NOT NULL DEFAULT '', -- CONSERVER | ELIMINER | ARCHIVER | VERSER_SAE
    metadata              JSONB NOT NULL DEFAULT '{}',

    CONSTRAINT record_metadata_confidentiality_range
        CHECK (confidentiality_level BETWEEN 0 AND 5)
);

CREATE INDEX idx_record_metadata_owner_user ON record_metadata (owner_user_id);
CREATE INDEX idx_record_metadata_owner_org ON record_metadata (owner_org_id);
CREATE INDEX idx_record_metadata_deleted_at ON record_metadata (deleted_at);

-- Append-only probative audit log --------------------------------------------
CREATE TABLE audit_event (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id     UUID NOT NULL REFERENCES subject_ref (id),
    event_type     TEXT NOT NULL,
    actor_user_id  TEXT NOT NULL DEFAULT '',
    occurred_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    before_state   JSONB,
    after_state    JSONB,
    reason         TEXT NOT NULL DEFAULT '',
    correlation_id UUID,
    request_id     TEXT NOT NULL DEFAULT '',
    metadata       JSONB NOT NULL DEFAULT '{}',

    CONSTRAINT audit_event_type_not_blank CHECK (length(btrim(event_type)) > 0)
);

CREATE INDEX idx_audit_event_subject ON audit_event (subject_id, occurred_at DESC);
CREATE INDEX idx_audit_event_type ON audit_event (event_type);
CREATE INDEX idx_audit_event_correlation ON audit_event (correlation_id);

-- migrate:down

DROP TABLE IF EXISTS audit_event;
DROP TABLE IF EXISTS record_metadata;
DROP TABLE IF EXISTS subject_ref;
DROP TABLE IF EXISTS subject_kind;
