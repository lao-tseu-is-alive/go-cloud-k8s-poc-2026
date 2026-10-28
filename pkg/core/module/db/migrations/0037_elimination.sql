-- migrate:up

-- Goéland POC — elimination of cases (roadmap GLD-077, v2 §35).
--
-- Executing an approved DESTROY bordereau erases the content of a case and of
-- the documents only it holds, keeping a minimal notice and the bordereau's
-- proof. Three tables are protected against any change by triggers (the audit
-- log, validated timeline entries, validated or record document versions);
-- they now accept one controlled exception: inside a transaction that declared
-- SET LOCAL goeland.disposal = 'on' (only the retention service does, while
-- executing an approved elimination), an UPDATE may blank their content. Their
-- identity (who, when, which subject, which file), deletions and TRUNCATE stay
-- refused. A role allowed to run DDL can still drop the triggers (GLD-046).

-- A blob whose bytes were destroyed; an identical upload stores them again.
ALTER TABLE content_blob ADD COLUMN disposed_at TIMESTAMPTZ;

-- migrate:statementbegin
CREATE OR REPLACE FUNCTION disposal_in_progress() RETURNS boolean AS $$
    SELECT coalesce(current_setting('goeland.disposal', true), '') = 'on';
$$ LANGUAGE sql STABLE;
-- migrate:statementend

-- The audit log: append-only, except blanking the states and reason of an
-- event during an elimination.
-- migrate:statementbegin
CREATE OR REPLACE FUNCTION guard_audit_event() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'UPDATE' AND disposal_in_progress()
        AND NEW.id = OLD.id AND NEW.subject_id = OLD.subject_id AND NEW.event_type = OLD.event_type
        AND NEW.actor_user_id = OLD.actor_user_id AND NEW.occurred_at = OLD.occurred_at THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION '% is append-only: % refused', TG_TABLE_NAME, TG_OP
        USING ERRCODE = 'integrity_constraint_violation';
END;
$$ LANGUAGE plpgsql;
-- migrate:statementend

DROP TRIGGER trg_audit_event_append_only ON audit_event;
CREATE TRIGGER trg_audit_event_append_only
    BEFORE UPDATE OR DELETE ON audit_event
    FOR EACH ROW
    EXECUTE FUNCTION guard_audit_event();

-- Timeline entries: only a draft changes, except blanking the texts of any
-- entry during an elimination.
-- migrate:statementbegin
CREATE OR REPLACE FUNCTION guard_case_timeline_entry() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'case_timeline_entry rows are never deleted (entry %)', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    IF NEW.case_id <> OLD.case_id OR NEW.created_at <> OLD.created_at OR NEW.created_by <> OLD.created_by
        OR NEW.corrects_entry_id IS DISTINCT FROM OLD.corrects_entry_id THEN
        RAISE EXCEPTION 'case_timeline_entry % identity is immutable', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    IF OLD.status <> 1 AND NOT (disposal_in_progress() AND NEW.status = OLD.status) THEN
        RAISE EXCEPTION 'case_timeline_entry % is no longer a draft and is immutable', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- migrate:statementend

-- Document versions: append-only and immutable once validated, except blanking
-- their metadata during an elimination; their content reference never changes.
-- migrate:statementbegin
CREATE OR REPLACE FUNCTION guard_document_version() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'document_version rows are never deleted (version %)', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    IF NEW.document_id <> OLD.document_id OR NEW.version_no <> OLD.version_no
        OR NEW.content_blob_id IS DISTINCT FROM OLD.content_blob_id
        OR NEW.created_at <> OLD.created_at OR NEW.created_by <> OLD.created_by THEN
        RAISE EXCEPTION 'document_version % identity and content are immutable', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    IF (OLD.validated_at IS NOT NULL OR OLD.is_record) AND NOT disposal_in_progress() THEN
        RAISE EXCEPTION 'document_version % is validated or a record and is immutable', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- migrate:statementend

-- migrate:down

-- migrate:statementbegin
CREATE OR REPLACE FUNCTION guard_document_version() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'document_version rows are never deleted (version %)', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    IF OLD.validated_at IS NOT NULL OR OLD.is_record THEN
        RAISE EXCEPTION 'document_version % is validated or a record and is immutable', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    IF NEW.document_id <> OLD.document_id OR NEW.version_no <> OLD.version_no
        OR NEW.content_blob_id IS DISTINCT FROM OLD.content_blob_id
        OR NEW.created_at <> OLD.created_at OR NEW.created_by <> OLD.created_by THEN
        RAISE EXCEPTION 'document_version % identity and content are immutable', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- migrate:statementend

-- migrate:statementbegin
CREATE OR REPLACE FUNCTION guard_case_timeline_entry() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'case_timeline_entry rows are never deleted (entry %)', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    IF OLD.status <> 1 THEN
        RAISE EXCEPTION 'case_timeline_entry % is no longer a draft and is immutable', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    IF NEW.case_id <> OLD.case_id OR NEW.created_at <> OLD.created_at OR NEW.created_by <> OLD.created_by
        OR NEW.corrects_entry_id IS DISTINCT FROM OLD.corrects_entry_id THEN
        RAISE EXCEPTION 'case_timeline_entry % identity is immutable', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- migrate:statementend

DROP TRIGGER trg_audit_event_append_only ON audit_event;
CREATE TRIGGER trg_audit_event_append_only
    BEFORE UPDATE OR DELETE ON audit_event
    FOR EACH ROW
    EXECUTE FUNCTION refuse_append_only_change();
DROP FUNCTION IF EXISTS guard_audit_event();
DROP FUNCTION IF EXISTS disposal_in_progress();
ALTER TABLE content_blob DROP COLUMN IF EXISTS disposed_at;
