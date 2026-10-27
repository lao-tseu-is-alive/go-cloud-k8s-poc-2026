package integration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// recordingConsumer counts the deliveries of the events of its subjects and
// refuses (by panicking) those of a subject while failOn holds it.
type recordingConsumer struct {
	mu        sync.Mutex
	subjects  map[uuid.UUID]bool
	delivered map[uuid.UUID]int
	failOn    uuid.UUID
}

func newRecordingConsumer(subjects ...uuid.UUID) *recordingConsumer {
	c := &recordingConsumer{subjects: map[uuid.UUID]bool{}, delivered: map[uuid.UUID]int{}}
	for _, id := range subjects {
		c.subjects[id] = true
	}
	return c
}

func (c *recordingConsumer) Name() string { return "it-recorder" }

func (c *recordingConsumer) Handle(_ context.Context, ev *core.OutboxEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ev.SubjectID == c.failOn {
		panic("refused on purpose")
	}
	if c.subjects[ev.SubjectID] {
		c.delivered[ev.ID]++
	}
	return nil
}

func (c *recordingConsumer) counts() map[uuid.UUID]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[uuid.UUID]int, len(c.delivered))
	for k, v := range c.delivered {
		out[k] = v
	}
	return out
}

// outboxOf returns the outbox events of a subject, oldest first.
func outboxOf(t *testing.T, env *testEnv, subjectID uuid.UUID) []*core.OutboxEvent {
	t.Helper()
	rows, err := env.pool.Query(env.ctx, `
SELECT id, subject_kind, subject_id, event_type, audit_event_id, payload, occurred_at, correlation_id, request_id,
       attempts, next_attempt_at, published_at, failed_at, last_error, failed_consumers
FROM outbox_event WHERE subject_id = $1 ORDER BY occurred_at, id`, subjectID)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	events, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[core.OutboxEvent])
	if err != nil {
		t.Fatalf("scan outbox: %v", err)
	}
	return events
}

func newOutboxWorker(t *testing.T, env *testEnv, consumers ...core.OutboxConsumer) *core.OutboxWorker {
	t.Helper()
	w, err := core.NewOutboxWorker(env.pool, core.OutboxConfig{Batch: 500}, nil, consumers...)
	if err != nil {
		t.Fatalf("outbox worker: %v", err)
	}
	return w
}

// TestOutboxEvents covers GLD-028: every audit event writes its outbox event
// in the same transaction (none when it rolls back), the worker delivers them
// and publishes them, and the delivered events are purged after the retention.
func TestOutboxEvents(t *testing.T) {
	env := newTestEnv(t)
	created, _, err := env.caseSvc.Create(env.ctx, caseInput("Outbox "+uniqueToken(), 0))
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	var audits, missing int
	if err := env.pool.QueryRow(env.ctx, `
SELECT count(*), count(*) FILTER (WHERE NOT EXISTS (SELECT 1 FROM outbox_event o WHERE o.audit_event_id = a.id))
FROM audit_event a WHERE a.subject_id = $1`, created.ID).Scan(&audits, &missing); err != nil {
		t.Fatal(err)
	}
	events := outboxOf(t, env, created.ID)
	if audits == 0 || missing != 0 || len(events) != audits {
		t.Fatalf("one outbox event per audit event: %d audits, %d without event, %d events", audits, missing, len(events))
	}
	if ev := events[0]; ev.SubjectKind != core.SubjectKindCase || ev.Payload["eventType"] != ev.EventType || ev.PublishedAt != nil {
		t.Fatalf("the event names its subject and waits for delivery: %+v", ev)
	}

	rolledBack := "IT_ROLLBACK_" + uniqueToken()
	err = core.InTx(env.ctx, env.pool, "rolled back", func(tx pgx.Tx) error {
		if _, err := core.InsertAuditEventTx(env.ctx, tx, core.AuditEvent{SubjectID: created.ID, EventType: rolledBack, ActorUserID: testOperator}); err != nil {
			return err
		}
		return errors.New("roll back")
	})
	if err == nil {
		t.Fatal("the transaction must fail")
	}
	var leaked int
	if err := env.pool.QueryRow(env.ctx, `SELECT count(*) FROM outbox_event WHERE event_type = $1`, rolledBack).Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("a rolled-back change announces nothing: %d (%v)", leaked, err)
	}

	consumer := newRecordingConsumer(created.ID)
	worker := newOutboxWorker(t, env, consumer)
	worker.Drain(env.ctx)
	for _, ev := range outboxOf(t, env, created.ID) {
		if consumer.counts()[ev.ID] != 1 || ev.PublishedAt == nil || ev.Attempts != 1 {
			t.Fatalf("each event is delivered once and published: %+v (delivered %d)", ev, consumer.counts()[ev.ID])
		}
	}

	if _, err := env.pool.Exec(env.ctx, `UPDATE outbox_event SET published_at = now() - interval '8 days' WHERE subject_id = $1`, created.ID); err != nil {
		t.Fatal(err)
	}
	if n, err := worker.Purge(env.ctx); err != nil || n < int64(len(events)) {
		t.Fatalf("the delivered events past the retention are purged: %d (%v)", n, err)
	}
	if left := outboxOf(t, env, created.ID); len(left) != 0 {
		t.Fatalf("purged events are gone: %d left", len(left))
	}
}

// TestOutboxRetries covers GLD-028: a refused delivery (here a panicking
// consumer) is retried later, given up after OutboxMaxAttempts, shown in the
// status and delivered again once replayed.
func TestOutboxRetries(t *testing.T) {
	env := newTestEnv(t)
	created, _, err := env.caseSvc.Create(env.ctx, caseInput("Outbox refusé "+uniqueToken(), 0))
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	consumer := newRecordingConsumer(created.ID)
	consumer.failOn = created.ID
	worker := newOutboxWorker(t, env, consumer)
	env.coreSvc.UseOutboxWorker(worker)

	worker.Drain(env.ctx)
	events := outboxOf(t, env, created.ID)
	for _, ev := range events {
		if ev.PublishedAt != nil || ev.FailedAt != nil || ev.Attempts != 1 || !ev.NextAttemptAt.After(time.Now()) ||
			len(ev.FailedConsumers) != 1 || ev.FailedConsumers[0] != "it-recorder" {
			t.Fatalf("a refused event waits for a later retry: %+v", ev)
		}
	}

	// The last attempt is due now.
	if _, err := env.pool.Exec(env.ctx, `UPDATE outbox_event SET attempts = $2, next_attempt_at = now() WHERE subject_id = $1`,
		created.ID, core.OutboxMaxAttempts-1); err != nil {
		t.Fatal(err)
	}
	worker.Drain(env.ctx)
	ids := make([]uuid.UUID, 0, len(events))
	for _, ev := range outboxOf(t, env, created.ID) {
		if ev.FailedAt == nil || ev.LastError == "" {
			t.Fatalf("the event is given up after the last attempt: %+v", ev)
		}
		ids = append(ids, ev.ID)
	}
	status, err := env.coreSvc.OutboxStatus(env.ctx)
	if err != nil {
		t.Fatalf("outbox status: %v", err)
	}
	if status.Failed < int32(len(ids)) || len(status.Consumers) != 1 || status.Consumers[0].Name != "it-recorder" || status.Consumers[0].Failed < int32(len(ids)) {
		t.Fatalf("the status counts the given-up events per consumer: %+v", status)
	}

	if _, err := env.coreSvc.ReplayOutbox(env.ctx, testOperator, nil, false); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("a replay names its events or asks for all: %v", err)
	}
	consumer.failOn = uuid.Nil
	if n, err := env.coreSvc.ReplayOutbox(env.ctx, testOperator, ids, false); err != nil || n != int32(len(ids)) {
		t.Fatalf("replay the given-up events: %d (%v)", n, err)
	}
	worker.Drain(env.ctx)
	for _, ev := range outboxOf(t, env, created.ID) {
		if ev.PublishedAt == nil || ev.Attempts != 1 || consumer.counts()[ev.ID] != 1 {
			t.Fatalf("a replayed event is delivered afresh: %+v", ev)
		}
	}
}

// TestOutboxConcurrentWorkers covers GLD-028: two workers draining the same
// queue never deliver an event twice.
func TestOutboxConcurrentWorkers(t *testing.T) {
	env := newTestEnv(t)
	created, _, err := env.caseSvc.Create(env.ctx, caseInput("Outbox parallèle "+uniqueToken(), 0))
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	const extra = 200
	err = core.InTx(env.ctx, env.pool, "many events", func(tx pgx.Tx) error {
		for range extra {
			if _, err := core.InsertAuditEventTx(env.ctx, tx, core.AuditEvent{SubjectID: created.ID, EventType: "IT_CONCURRENT", ActorUserID: testOperator}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("write events: %v", err)
	}
	consumer := newRecordingConsumer(created.ID)
	var wg sync.WaitGroup
	for range 2 {
		w, err := core.NewOutboxWorker(env.pool, core.OutboxConfig{Batch: 10}, nil, consumer)
		if err != nil {
			t.Fatal(err)
		}
		wg.Go(func() { w.Drain(env.ctx) })
	}
	wg.Wait()
	events := outboxOf(t, env, created.ID)
	if len(events) < extra {
		t.Fatalf("expected at least %d events, got %d", extra, len(events))
	}
	counts := consumer.counts()
	for _, ev := range events {
		if counts[ev.ID] != 1 || ev.PublishedAt == nil {
			t.Fatalf("event %s delivered %d times (published %v)", ev.ID, counts[ev.ID], ev.PublishedAt)
		}
	}
}
