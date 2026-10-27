package core

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// OutboxChannel is the PostgreSQL notification channel that wakes the outbox
// workers when an event is written or replayed (GLD-028).
const OutboxChannel = "goeland_outbox"

// Outbox delivery settings shared by the worker and the status RPC.
const (
	// OutboxMaxAttempts is how many deliveries are tried before an event is
	// given up (failed_at) and left for an administrator to replay.
	OutboxMaxAttempts = 10
	// DefaultOutboxRetention is how long delivered events are kept.
	DefaultOutboxRetention = 7 * 24 * time.Hour
	// outboxFirstRetry and outboxMaxRetry bound the delay before a retry: it
	// grows six-fold per attempt (10 s, 1 min, 6 min, 36 min, then hourly).
	outboxFirstRetry = 10 * time.Second
	outboxMaxRetry   = time.Hour
	// recentFailedLimit bounds the failed events listed by the status.
	recentFailedLimit = 50
)

// OutboxEvent is a change announced to the outbox consumers (GLD-028): one per
// audit event, written in the same transaction. It carries a reference only
// (subject, type, audit event); a consumer reads the current state itself, so
// a duplicate or a late delivery does no harm.
type OutboxEvent struct {
	// ID is the event's UUID.
	ID uuid.UUID `db:"id"`
	// SubjectKind is the kind of the audited subject (CASE, DOCUMENT...).
	SubjectKind SubjectKind `db:"subject_kind"`
	// SubjectID is the audited subject.
	SubjectID uuid.UUID `db:"subject_id"`
	// EventType is the audit event type (CASE_UPDATED, RELATIONSHIP_LINKED...).
	EventType string `db:"event_type"`
	// AuditEventID is the audit event behind it.
	AuditEventID uuid.UUID `db:"audit_event_id"`
	// Payload is the reference as JSON, ready to forward to a message broker.
	Payload map[string]any `db:"payload"`
	// OccurredAt is when the change was made.
	OccurredAt time.Time `db:"occurred_at"`
	// CorrelationID groups the events of one business operation, when set.
	CorrelationID *uuid.UUID `db:"correlation_id"`
	// RequestID is the HTTP request that made the change.
	RequestID string `db:"request_id"`
	// Attempts counts the deliveries tried, the current one included.
	Attempts int32 `db:"attempts"`
	// NextAttemptAt is when a pending event is tried again.
	NextAttemptAt time.Time `db:"next_attempt_at"`
	// PublishedAt is when every consumer accepted it.
	PublishedAt *time.Time `db:"published_at"`
	// FailedAt is when the delivery was given up.
	FailedAt *time.Time `db:"failed_at"`
	// LastError is the error of the last failed attempt, per consumer.
	LastError string `db:"last_error"`
	// FailedConsumers are the consumers whose last attempt failed.
	FailedConsumers []string `db:"failed_consumers"`
}

// OutboxConsumerStatus is the state of one registered consumer.
type OutboxConsumerStatus struct {
	// Name identifies the consumer.
	Name string `db:"name"`
	// Failed counts the given-up events it refused.
	Failed int32 `db:"failed"`
}

// OutboxStatus describes the outbox for its administrators.
type OutboxStatus struct {
	// Pending counts the events waiting for delivery or a retry.
	Pending int32 `db:"pending"`
	// Failed counts the given-up events.
	Failed int32 `db:"failed"`
	// Published counts the delivered events still kept.
	Published int32 `db:"published"`
	// OldestPendingAt is when the oldest pending event occurred.
	OldestPendingAt *time.Time `db:"oldest_pending_at"`
	// Consumers are the consumers registered on this server.
	Consumers []OutboxConsumerStatus
	// RecentFailed are the latest given-up events, newest first.
	RecentFailed []*OutboxEvent
}

// insertOutboxEventTx announces the audit event ev to the outbox consumers,
// in the transaction of q (called by InsertAuditEventTx only).
func insertOutboxEventTx(ctx context.Context, q Querier, ev *AuditEvent) error {
	_, err := q.Exec(ctx, insertOutboxEventSQL, pgx.NamedArgs{
		"subject_id":     ev.SubjectID,
		"event_type":     ev.EventType,
		"audit_event_id": ev.ID,
		"payload": map[string]any{
			"subjectId": ev.SubjectID, "eventType": ev.EventType, "auditEventId": ev.ID, "occurredAt": ev.OccurredAt,
		},
		"occurred_at":    ev.OccurredAt,
		"correlation_id": ev.CorrelationID,
		"request_id":     ev.RequestID,
	})
	return err
}

// OutboxRetryDelay is the wait before the next delivery of an event whose
// attempt-th delivery failed: 10 s, then six times longer each time, at most
// an hour.
func OutboxRetryDelay(attempt int32) time.Duration {
	delay := outboxFirstRetry
	for i := int32(1); i < attempt && delay < outboxMaxRetry; i++ {
		delay *= 6
	}
	return min(delay, outboxMaxRetry)
}

// UseOutboxWorker tells the service which consumers this server's outbox
// worker delivers to and how long it keeps delivered events, for the status.
// Call it once at wiring.
func (s *Service) UseOutboxWorker(w *OutboxWorker) {
	s.outboxConsumers, s.outboxRetention = w.ConsumerNames(), w.Retention()
}

// OutboxRetention is how long this server keeps delivered events.
func (s *Service) OutboxRetention() time.Duration {
	return orDefault(s.outboxRetention, DefaultOutboxRetention)
}

// OutboxStatus describes the outbox: counts, every registered consumer with
// the given-up events it refused (and those of consumers no longer
// registered), the latest given-up events.
func (s *Service) OutboxStatus(ctx context.Context) (OutboxStatus, error) {
	status, err := s.repo.OutboxStatus(ctx)
	if err != nil {
		return OutboxStatus{}, err
	}
	failed := map[string]int32{}
	for _, c := range status.Consumers {
		failed[c.Name] = c.Failed
	}
	consumers := make([]OutboxConsumerStatus, 0, len(s.outboxConsumers)+len(failed))
	for _, name := range s.outboxConsumers {
		consumers = append(consumers, OutboxConsumerStatus{Name: name, Failed: failed[name]})
		delete(failed, name)
	}
	for _, c := range status.Consumers {
		if _, gone := failed[c.Name]; gone {
			consumers = append(consumers, c)
		}
	}
	status.Consumers = consumers
	return status, nil
}

// ReplayOutbox puts failed events back in the queue (those of ids, or all of
// them when all) on behalf of operator, and returns how many.
func (s *Service) ReplayOutbox(ctx context.Context, operator string, ids []uuid.UUID, all bool) (int32, error) {
	if all == (len(ids) > 0) {
		return 0, fmt.Errorf("%w: give either event ids or all", ErrInvalidInput)
	}
	n, err := s.repo.ReplayOutbox(ctx, ids, all)
	if err != nil {
		return 0, err
	}
	s.log.Info("outbox events replayed", "operator", operator, "replayed", n, "all", all)
	return n, nil
}
