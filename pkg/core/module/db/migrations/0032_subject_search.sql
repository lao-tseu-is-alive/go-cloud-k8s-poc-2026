-- migrate:up

-- Goéland POC — global search over every subject (roadmap GLD-064).
--
-- subject_ref already holds one row per subject of every kind with its label
-- and business reference, kept current by core (InsertSubjectRefTx,
-- UpdateSubjectLabelTx) and by the legacy import: it is the search table, so
-- the global search needs no copy of the data, only an index fitting its
-- query (core.subjectSearchVectorSQL must stay identical to this expression):
--
--   - label and reference, accent-folded, every non-alphanumeric character
--     folded to a space, so the indexed words are exactly those the query side
--     splits (core.QueryWords): "2026-000123" is "2026" and "000123", where the
--     parser alone reads "-000123" as a signed number;
--   - plus one token naming the kind ('§case', '§document', ...), which the user
--     text can never produce, so a per-kind search reads only that kind's
--     matches from the index instead of every kind's.
--
-- The previous label index (not accent-folded, no reference) was used by no
-- query; this one replaces it. Building it takes about a minute on 3.1M subjects.

DROP INDEX IF EXISTS idx_subject_ref_display_label;

CREATE INDEX idx_subject_ref_search ON subject_ref USING gin ((
    to_tsvector('simple', regexp_replace(immutable_unaccent(display_label || ' ' || business_ref), '[^[:alnum:]]+', ' ', 'g'))
    || ('''§' || lower(kind) || '''')::tsvector
));

-- migrate:down

DROP INDEX IF EXISTS idx_subject_ref_search;

CREATE INDEX idx_subject_ref_display_label
    ON subject_ref USING gin (to_tsvector('simple', display_label));
