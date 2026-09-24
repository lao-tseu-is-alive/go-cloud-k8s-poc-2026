-- migrate:up

-- Goéland POC — ending a relationship (IMPLEMENTATION_STATUS §3g, roadmap GLD-034).
--
-- "Ended" (valid_to set by EndRelationship) and "unlinked" (deleted_at, the edge
-- was a mistake) are two distinct facts. An ended edge stays in the graph as
-- history, so uniqueness now applies to *open* edges only: once a mandate has
-- ended, the same actor can be linked again in the same role.

DROP INDEX IF EXISTS idx_subject_relationship_active_unique;

CREATE UNIQUE INDEX idx_subject_relationship_open_unique
    ON subject_relationship (source_subject_id, target_subject_id, relationship_type_id)
    WHERE deleted_at IS NULL AND valid_to IS NULL;

ALTER TABLE subject_relationship
    ADD CONSTRAINT subject_relationship_validity_ordered
    CHECK (valid_from IS NULL OR valid_to IS NULL OR valid_to >= valid_from);

-- migrate:down

ALTER TABLE subject_relationship DROP CONSTRAINT IF EXISTS subject_relationship_validity_ordered;

DROP INDEX IF EXISTS idx_subject_relationship_open_unique;

-- Restoring the stricter index fails while ended and open duplicates coexist;
-- a clean rollback assumes no such history, as for the other seed rollbacks.
CREATE UNIQUE INDEX idx_subject_relationship_active_unique
    ON subject_relationship (source_subject_id, target_subject_id, relationship_type_id)
    WHERE deleted_at IS NULL;
