-- migrate:up

-- Goéland POC — pre-registered users (roadmap GLD-093).
--
-- An administrator may register a new employee before their first sign-in, so
-- units, groups and roles can be given at once. Such a user has no sign-in yet:
-- first_seen_at and last_seen_at become nullable (NULL until the first
-- sign-in, which sets both), and registered_by keeps the administrator. The
-- identity stays the directory's: the first sign-in overwrites the name and
-- e-mail with the token's (USER_PROFILE_UPDATED when they differ).

ALTER TABLE app_user
    ALTER COLUMN first_seen_at DROP NOT NULL,
    ALTER COLUMN last_seen_at DROP NOT NULL,
    ADD COLUMN registered_by TEXT NOT NULL DEFAULT '';

-- migrate:down

UPDATE app_user SET first_seen_at = coalesce(first_seen_at, now()), last_seen_at = coalesce(last_seen_at, now());
ALTER TABLE app_user
    DROP COLUMN registered_by,
    ALTER COLUMN first_seen_at SET NOT NULL,
    ALTER COLUMN last_seen_at SET NOT NULL;
