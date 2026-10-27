-- migrate:up

-- Goéland POC — indexes for the creator filter of the lists (roadmap GLD-074).
--
-- The case, document, thing and actor searches filter on created_by
-- (core.RecordFilterSQL). Measured on the imported data, without an index a
-- creator's page walks the newest rows until it finds 25 of theirs (about
-- 100 ms on the 2.3M documents for an average creator, a full scan for one
-- whose records are old); with these indexes it reads them directly (about
-- 1 ms). Cases and documents keep the default order (newest first) in the
-- index; thing and actor lists are short per creator.

CREATE INDEX idx_case_file_creator ON case_file (created_by, created_at DESC, id DESC);
CREATE INDEX idx_document_creator ON document (created_by, created_at DESC, id DESC);
CREATE INDEX idx_thing_creator ON thing (created_by);
CREATE INDEX idx_actor_creator ON actor (created_by);

-- migrate:down

DROP INDEX IF EXISTS idx_actor_creator;
DROP INDEX IF EXISTS idx_thing_creator;
DROP INDEX IF EXISTS idx_document_creator;
DROP INDEX IF EXISTS idx_case_file_creator;
