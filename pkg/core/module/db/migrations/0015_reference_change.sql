-- migrate:up

-- Goéland POC — reference data administration (roadmap GLD-040).
--
-- Reference catalogues (case_type, relationship_type, organization_category,
-- document_type) are not subjects, so their changes cannot go to audit_event
-- (which references subject_ref). reference_change is their append-only log:
-- one row per creation or update, with the before/after state, the operator
-- and the reason. Catalogue rows are never deleted, only deactivated.

CREATE TABLE reference_change (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    catalogue     TEXT NOT NULL,
    code          TEXT NOT NULL,
    event_type    TEXT NOT NULL,
    actor_user_id TEXT NOT NULL DEFAULT '',
    occurred_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    before_state  JSONB,
    after_state   JSONB,
    reason        TEXT NOT NULL DEFAULT '',

    CONSTRAINT reference_change_catalogue_valid CHECK (
        catalogue IN ('case_type', 'relationship_type', 'organization_category', 'document_type')
    ),
    CONSTRAINT reference_change_event_valid CHECK (event_type IN ('REFERENCE_CREATED', 'REFERENCE_UPDATED'))
);

CREATE INDEX idx_reference_change_recent ON reference_change (occurred_at DESC);
CREATE INDEX idx_reference_change_entry ON reference_change (catalogue, code, occurred_at DESC);

-- migrate:down

DROP TABLE IF EXISTS reference_change;
