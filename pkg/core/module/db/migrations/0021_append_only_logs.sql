-- migrate:up

-- The audit trail (audit_event) and the reference data change log
-- (reference_change) are append-only: the application only ever inserts into
-- them, and their probative value requires that no row be rewritten or removed
-- afterwards, not even by a direct SQL session of the application role
-- (review 2026-09-30, GLD-044). A role allowed to run DDL can still drop these
-- triggers; separating the migration role from the runtime role is GLD-046.

-- migrate:statementbegin
CREATE OR REPLACE FUNCTION refuse_append_only_change() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION '% is append-only: % refused', TG_TABLE_NAME, TG_OP
        USING ERRCODE = 'integrity_constraint_violation';
END;
$$ LANGUAGE plpgsql;
-- migrate:statementend

CREATE TRIGGER trg_audit_event_append_only
    BEFORE UPDATE OR DELETE ON audit_event
    FOR EACH ROW
    EXECUTE FUNCTION refuse_append_only_change();

CREATE TRIGGER trg_audit_event_no_truncate
    BEFORE TRUNCATE ON audit_event
    FOR EACH STATEMENT
    EXECUTE FUNCTION refuse_append_only_change();

CREATE TRIGGER trg_reference_change_append_only
    BEFORE UPDATE OR DELETE ON reference_change
    FOR EACH ROW
    EXECUTE FUNCTION refuse_append_only_change();

CREATE TRIGGER trg_reference_change_no_truncate
    BEFORE TRUNCATE ON reference_change
    FOR EACH STATEMENT
    EXECUTE FUNCTION refuse_append_only_change();

-- migrate:down

DROP TRIGGER IF EXISTS trg_reference_change_no_truncate ON reference_change;
DROP TRIGGER IF EXISTS trg_reference_change_append_only ON reference_change;
DROP TRIGGER IF EXISTS trg_audit_event_no_truncate ON audit_event;
DROP TRIGGER IF EXISTS trg_audit_event_append_only ON audit_event;
DROP FUNCTION IF EXISTS refuse_append_only_change();
