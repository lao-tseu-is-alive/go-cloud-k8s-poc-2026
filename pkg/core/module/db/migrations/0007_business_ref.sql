-- migrate:up

-- Goéland POC — business reference on every subject (spec v2 §8, roadmap GLD-022).
--
-- The UUID stays the immutable technical identity. A subject may additionally
-- carry a human business reference (e.g. "2026-001245"), optionally scoped by a
-- namespace (e.g. "OPC"):
--   * with a namespace the pair (namespace, business_ref) is unique;
--   * without a namespace the reference is free text (e.g. an imported legacy
--     number) and is NOT unique;
--   * a namespace without a reference is meaningless and rejected.
-- The display label is not an identifier and stays non-unique.

ALTER TABLE subject_ref
    ADD COLUMN business_ref           TEXT NOT NULL DEFAULT '',
    ADD COLUMN business_ref_namespace TEXT NOT NULL DEFAULT '',
    ADD CONSTRAINT subject_ref_business_ref_namespace_needs_ref
        CHECK (business_ref_namespace = '' OR business_ref <> '');

CREATE UNIQUE INDEX idx_subject_ref_business_ref_unique
    ON subject_ref (business_ref_namespace, business_ref)
    WHERE business_ref_namespace <> '';

CREATE INDEX idx_subject_ref_business_ref
    ON subject_ref (business_ref)
    WHERE business_ref <> '';

-- Allocator of server-generated references "<period>-<NNNNNN>". One row per
-- (namespace, period); the application increments it with
-- INSERT ... ON CONFLICT DO UPDATE ... RETURNING, which row-locks the counter for
-- the rest of the transaction, so concurrent allocations are serialized and a
-- rolled-back transaction gives its number back (no gaps from failures).
CREATE TABLE business_ref_counter (
    namespace  TEXT NOT NULL,
    period     TEXT NOT NULL, -- calendar year in Europe/Zurich, e.g. '2026'
    last_value BIGINT NOT NULL,

    PRIMARY KEY (namespace, period),
    CONSTRAINT business_ref_counter_namespace_not_blank CHECK (length(btrim(namespace)) > 0),
    CONSTRAINT business_ref_counter_positive CHECK (last_value > 0)
);

-- migrate:down

DROP TABLE IF EXISTS business_ref_counter;
DROP INDEX IF EXISTS idx_subject_ref_business_ref;
DROP INDEX IF EXISTS idx_subject_ref_business_ref_unique;
ALTER TABLE subject_ref
    DROP CONSTRAINT IF EXISTS subject_ref_business_ref_namespace_needs_ref,
    DROP COLUMN IF EXISTS business_ref_namespace,
    DROP COLUMN IF EXISTS business_ref;
