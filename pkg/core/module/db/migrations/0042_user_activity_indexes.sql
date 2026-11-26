-- migrate:up

-- Goéland POC — the caller's activity (roadmap GLD-088, « Mon tableau de bord »).
--
-- The dashboard lists the subjects a user changed (audit_event.actor_user_id)
-- or consulted (access_audit_event.user_id, GLD-089) and sorts them by the
-- user's last activity: both lookups go by user, then subject, then time.

CREATE INDEX idx_audit_event_actor_subject ON audit_event (actor_user_id, subject_id, occurred_at DESC);
CREATE INDEX idx_access_audit_event_user_subject ON access_audit_event (user_id, subject_id, occurred_at DESC);

-- migrate:down

DROP INDEX IF EXISTS idx_access_audit_event_user_subject;
DROP INDEX IF EXISTS idx_audit_event_actor_subject;
