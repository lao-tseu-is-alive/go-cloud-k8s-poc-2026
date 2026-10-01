-- migrate:up

-- Goéland POC — import batches and subject provenance (roadmap GLD-027, GLD-051).
--
-- An import run is one import_batch row: where it read from, the date of the
-- data (the instant T), what it wrote and what it left out, as counts. It is the
-- single marker of a set-based load, which writes no audit event per row.
-- subject_provenance ties a subject to the row it came from in another system;
-- the source ids are provenance, never the subject ids (those are computed,
-- UUIDv5 of "<KIND>:<source id>", so a rebuild gives the same ids).

CREATE TABLE import_batch (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_system   TEXT NOT NULL,
    -- snapshot_at is the date of the source data, when known (the instant T).
    snapshot_at     TIMESTAMPTZ,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at     TIMESTAMPTZ,
    started_by      TEXT NOT NULL,
    -- counts holds, per stage, the rows read, written and left out with the reason.
    counts          JSONB NOT NULL DEFAULT '{}'::jsonb,

    CONSTRAINT import_batch_source_not_blank CHECK (length(btrim(source_system)) > 0)
);

CREATE TABLE subject_provenance (
    subject_id      UUID PRIMARY KEY REFERENCES subject_ref (id),
    source_system   TEXT NOT NULL,
    source_table    TEXT NOT NULL,
    source_id       TEXT NOT NULL,
    import_batch_id UUID NOT NULL REFERENCES import_batch (id),

    CONSTRAINT subject_provenance_source_unique UNIQUE (source_system, source_table, source_id)
);

CREATE INDEX idx_subject_provenance_batch ON subject_provenance (import_batch_id);

-- migrate:down

DROP TABLE IF EXISTS subject_provenance;
DROP TABLE IF EXISTS import_batch;
