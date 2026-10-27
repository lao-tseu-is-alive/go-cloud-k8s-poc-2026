-- migrate:up

-- Goéland POC — transactional outbox (roadmap GLD-028, spec v2 §38).
--
-- Every audit event writes an outbox event in the same transaction
-- (core.InsertAuditEventTx): a change is announced if and only if it was
-- committed. A worker on every replica delivers the events to the registered
-- consumers (search index, notifications...) at least once, retries a failure
-- with a growing delay, gives up after a number of attempts (failed_at, kept
-- for an administrator to replay) and deletes the delivered events after the
-- retention period. The event carries a reference only: a consumer reads the
-- current state itself, so nothing confidential is copied here. This is not
-- event sourcing: the tables stay the truth, the audit stays the history.
-- audit_event_id has no foreign key: the audit is append-only already, and a
-- reference would put the outbox in the way of the audit's own guards (its
-- TRUNCATE refusal).

CREATE TABLE outbox_event (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_kind     TEXT NOT NULL DEFAULT '',
    subject_id       UUID NOT NULL,
    event_type       TEXT NOT NULL,
    audit_event_id   UUID NOT NULL,
    payload          JSONB NOT NULL DEFAULT '{}',
    occurred_at      TIMESTAMPTZ NOT NULL,
    correlation_id   UUID,
    request_id       TEXT NOT NULL DEFAULT '',
    attempts         INT NOT NULL DEFAULT 0,
    next_attempt_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    claimed_at       TIMESTAMPTZ,
    published_at     TIMESTAMPTZ,
    failed_at        TIMESTAMPTZ,
    last_error       TEXT NOT NULL DEFAULT '',
    failed_consumers TEXT[] NOT NULL DEFAULT '{}',
    CONSTRAINT outbox_event_one_outcome CHECK (published_at IS NULL OR failed_at IS NULL)
);

-- The queue: only the events still to deliver, in the order they occurred.
CREATE INDEX idx_outbox_event_pending ON outbox_event (occurred_at, id)
    WHERE published_at IS NULL AND failed_at IS NULL;
-- The purge of the delivered events and the list of the failed ones.
CREATE INDEX idx_outbox_event_published ON outbox_event (published_at) WHERE published_at IS NOT NULL;
CREATE INDEX idx_outbox_event_failed ON outbox_event (failed_at DESC) WHERE failed_at IS NOT NULL;
-- From an audit event to its announcement.
CREATE INDEX idx_outbox_event_audit ON outbox_event (audit_event_id);

-- migrate:down

DROP TABLE IF EXISTS outbox_event;
