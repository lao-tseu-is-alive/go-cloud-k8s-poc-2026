-- migrate:up

-- Goéland POC — internal users (roadmap GLD-025, spec v2 §5.7).
--
-- A USER is an authenticated internal identity (an employee), never an ACTOR.
-- The auth service owns accounts; this table only mirrors what the verified
-- token says about a user the first time and whenever it changes, so that
-- governance and audit (which store the operator id) can show a name, and so
-- that tasks can later target a USER subject through typed relationships.
--
-- user_id is the operator id recorded everywhere else (record_metadata.created_by,
-- audit_event.actor_user_id, ...): today the auth service's numeric user id as text.

CREATE TABLE app_user (
    user_id       TEXT PRIMARY KEY,
    subject_id    UUID NOT NULL UNIQUE,
    kind          TEXT NOT NULL DEFAULT 'USER',
    display_name  TEXT NOT NULL DEFAULT '',
    email         TEXT NOT NULL DEFAULT '',
    is_admin      BOOLEAN NOT NULL DEFAULT false,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT app_user_subject_fkey FOREIGN KEY (subject_id, kind) REFERENCES subject_ref (id, kind),
    CONSTRAINT app_user_kind_is_user CHECK (kind = 'USER'),
    CONSTRAINT app_user_id_not_blank CHECK (length(btrim(user_id)) > 0)
);

-- migrate:down

DROP TABLE IF EXISTS app_user;
