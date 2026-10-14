-- migrate:up

-- Goéland POC — non-destructive contact replacement (roadmap GLD-007) and
-- relationship kind rules in the database (roadmap GLD-060).
--
-- GLD-007: replacing an actor's contacts deleted the rows, so the previous
-- phone numbers and identifiers were lost (v2 §35 asks for non-destructive
-- changes). Like actor_address (0014), a replaced contact is now ended
-- (ended_at, ended_by) and kept; only rows without ended_at are current.
-- Rows existing before this migration are all current.
--
-- GLD-060: the source and target kinds of an edge must be those of its
-- relationship_type. core.LinkSubjectsTx checks it, but the legacy import and
-- any future worker write subject_relationship directly; the trigger keeps them
-- honest. Relationship type kinds are immutable (UpdateRelationshipType never
-- changes them), so checking the edge on write is enough.

ALTER TABLE actor_contact
    ADD COLUMN ended_at TIMESTAMPTZ,
    ADD COLUMN ended_by TEXT NOT NULL DEFAULT '';

-- the current contacts of an actor (every read); the value lookup keeps all rows
DROP INDEX IF EXISTS idx_actor_contact_actor;
CREATE INDEX idx_actor_contact_actor ON actor_contact (actor_id) WHERE ended_at IS NULL;

-- migrate:statementbegin
CREATE OR REPLACE FUNCTION check_relationship_kinds() RETURNS trigger AS $$
DECLARE
    expected_source TEXT;
    expected_target TEXT;
    actual_source   TEXT;
    actual_target   TEXT;
BEGIN
    SELECT source_kind, target_kind INTO expected_source, expected_target
    FROM relationship_type WHERE id = NEW.relationship_type_id;
    SELECT kind INTO actual_source FROM subject_ref WHERE id = NEW.source_subject_id;
    SELECT kind INTO actual_target FROM subject_ref WHERE id = NEW.target_subject_id;
    IF actual_source IS DISTINCT FROM expected_source OR actual_target IS DISTINCT FROM expected_target THEN
        RAISE EXCEPTION 'relationship % links % to %, its type expects % to %',
            NEW.id, actual_source, actual_target, expected_source, expected_target
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- migrate:statementend

CREATE TRIGGER trg_subject_relationship_kinds
    BEFORE INSERT OR UPDATE OF source_subject_id, target_subject_id, relationship_type_id
    ON subject_relationship
    FOR EACH ROW
    EXECUTE FUNCTION check_relationship_kinds();

-- migrate:down

DROP TRIGGER IF EXISTS trg_subject_relationship_kinds ON subject_relationship;
DROP FUNCTION IF EXISTS check_relationship_kinds();

-- ended contacts cannot come back as current ones: the rollback drops them
DELETE FROM actor_contact WHERE ended_at IS NOT NULL;
DROP INDEX IF EXISTS idx_actor_contact_actor;
CREATE INDEX idx_actor_contact_actor ON actor_contact (actor_id);
ALTER TABLE actor_contact
    DROP COLUMN ended_by,
    DROP COLUMN ended_at;
