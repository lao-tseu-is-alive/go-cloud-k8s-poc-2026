-- migrate:up

-- Goéland POC — indexes on the order of the searches and lists (roadmap GLD-053).
--
-- Measured on production volume (~512k cases, ~2.8M relationships): the
-- searches read their matches in this order and stop at the count limit
-- (core.CappedPageSQL), so an index on the sort key ends the scan early instead
-- of sorting every match. The relationship lists of one subject are ordered by
-- creation date within that subject.

CREATE INDEX idx_case_file_created ON case_file (created_at DESC, id DESC);
CREATE INDEX idx_document_created ON document (created_at DESC, id DESC);
CREATE INDEX idx_thing_created ON thing (created_at DESC, id DESC);
CREATE INDEX idx_actor_display_name ON actor (display_name, id);
CREATE INDEX idx_subject_relationship_source_created ON subject_relationship (source_subject_id, created_at DESC, id DESC);
CREATE INDEX idx_subject_relationship_target_created ON subject_relationship (target_subject_id, created_at DESC, id DESC);

-- migrate:down

DROP INDEX IF EXISTS idx_subject_relationship_target_created;
DROP INDEX IF EXISTS idx_subject_relationship_source_created;
DROP INDEX IF EXISTS idx_actor_display_name;
DROP INDEX IF EXISTS idx_thing_created;
DROP INDEX IF EXISTS idx_document_created;
DROP INDEX IF EXISTS idx_case_file_created;
