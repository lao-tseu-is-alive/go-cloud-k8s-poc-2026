-- migrate:up

-- Goéland POC — indexes on the sortable columns of the lists (roadmap GLD-055).
--
-- A list sorted on a column reads its matches in that order and stops at the
-- count limit (core.CappedPageSQL): on ~512k cases, sorting by title takes
-- ~0.9 s without an index and ~0.08 s with one. Low-cardinality columns
-- (status, kind, type) and joined labels sort fast enough without one.

CREATE INDEX idx_case_file_title ON case_file (title, id);
CREATE INDEX idx_case_file_opened ON case_file (opened_at, id);
CREATE INDEX idx_document_title_order ON document (title, id);
CREATE INDEX idx_document_official_date ON document (official_date, id);
CREATE INDEX idx_thing_name ON thing (name, id);
-- the case list sorts business references by value (numeric legacy numbers),
-- within the cases (the other subjects, mostly without a reference, come first otherwise)
CREATE INDEX idx_subject_ref_business_ref_order ON subject_ref (kind, lpad(business_ref, 20, '0'), id);
CREATE INDEX idx_document_status_order ON document (status, id);
-- a nullable key sorted descending puts NULL last, which an ascending index cannot serve
CREATE INDEX idx_document_official_date_desc ON document (official_date DESC NULLS LAST, id DESC);

-- migrate:down

DROP INDEX IF EXISTS idx_document_official_date_desc;
DROP INDEX IF EXISTS idx_document_status_order;
DROP INDEX IF EXISTS idx_subject_ref_business_ref_order;
DROP INDEX IF EXISTS idx_thing_name;
DROP INDEX IF EXISTS idx_document_official_date;
DROP INDEX IF EXISTS idx_document_title_order;
DROP INDEX IF EXISTS idx_case_file_opened;
DROP INDEX IF EXISTS idx_case_file_title;
