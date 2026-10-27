package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OutboxConsumer receives the outbox events (GLD-028): a search indexer, a
// notifier, a bridge to a message broker. Delivery is at least once and in
// no guaranteed order across subjects: Handle may see an event twice and
// should read the current state of ev.SubjectID rather than trust the event
// order. An error makes the worker deliver the event again later, to every
// consumer, so a consumer must be idempotent.
type OutboxConsumer interface {
	// Name identifies the consumer in the status and the errors; unique.
	Name() string
	// Handle processes one event.
	Handle(ctx context.Context, ev *OutboxEvent) error
}

// OutboxConfig tunes the outbox worker; zero values take the defaults.
type OutboxConfig struct {
	// Retention is how long delivered events are kept (default 7 days).
	Retention time.Duration
	// PollInterval is how often the worker looks for due events when no
	// notification wakes it (default 5 s): retries and missed notifications.
	PollInterval time.Duration
	// Batch is how many events a worker claims at once (default 100).
	Batch int
	// Lease is how long a claimed event stays reserved before another worker
	// may take it over (default 5 min).
	Lease time.Duration
}

const (
	defaultOutboxPoll  = 5 * time.Second
	defaultOutboxBatch = 100
	defaultOutboxLease = 5 * time.Minute
	// outboxPurgeEvery spaces the purges of delivered events.
	outboxPurgeEvery = 10 * time.Minute
	outboxPurgeBatch = 10000
	// maxOutboxError bounds the error text kept on an event.
	maxOutboxError = 2000
	// outboxRequeueTimeout bounds the release of the claims on shutdown.
	outboxRequeueTimeout = 5 * time.Second
)

// requeueOutboxEventsSQL releases the claims of a delivery interrupted by a
// shutdown, without counting it as an attempt.
const requeueOutboxEventsSQL = `
UPDATE outbox_event
SET claimed_at = NULL, attempts = greatest(attempts - 1, 0)
WHERE id = ANY(@ids) AND published_at IS NULL AND failed_at IS NULL;`

// OutboxWorker delivers the outbox events to the consumers (GLD-028). Every
// replica runs one; they share the queue through SKIP LOCKED claims. A
// notification on OutboxChannel wakes it as soon as a change commits, and it
// polls as a fallback.
type OutboxWorker struct {
	pool      *pgxpool.Pool
	cfg       OutboxConfig
	log       *slog.Logger
	consumers []OutboxConsumer
	lastPurge time.Time
}

// NewOutboxWorker builds a worker delivering to consumers, whose names must
// be unique and not empty. A nil logger falls back to slog.Default.
func NewOutboxWorker(pool *pgxpool.Pool, cfg OutboxConfig, log *slog.Logger, consumers ...OutboxConsumer) (*OutboxWorker, error) {
	if pool == nil {
		return nil, fmt.Errorf("%w: pool is required", ErrInvalidInput)
	}
	seen := map[string]bool{}
	for _, c := range consumers {
		if c == nil || c.Name() == "" || seen[c.Name()] {
			return nil, fmt.Errorf("%w: outbox consumers need distinct, non-empty names", ErrInvalidInput)
		}
		seen[c.Name()] = true
	}
	if log == nil {
		log = slog.Default()
	}
	cfg.Retention = orDefault(cfg.Retention, DefaultOutboxRetention)
	cfg.PollInterval = orDefault(cfg.PollInterval, defaultOutboxPoll)
	cfg.Lease = orDefault(cfg.Lease, defaultOutboxLease)
	if cfg.Batch <= 0 {
		cfg.Batch = defaultOutboxBatch
	}
	return &OutboxWorker{pool: pool, cfg: cfg, log: log, consumers: consumers}, nil
}

// orDefault returns d, or def when d is not positive.
func orDefault(d, def time.Duration) time.Duration {
	if d <= 0 {
		return def
	}
	return d
}

// ConsumerNames lists the registered consumers, in registration order.
func (w *OutboxWorker) ConsumerNames() []string {
	names := make([]string, len(w.consumers))
	for i, c := range w.consumers {
		names[i] = c.Name()
	}
	return names
}

// Retention is how long delivered events are kept.
func (w *OutboxWorker) Retention() time.Duration { return w.cfg.Retention }

// Run delivers events until ctx is cancelled: on every notification, on every
// poll, and purges the delivered events past the retention now and then.
func (w *OutboxWorker) Run(ctx context.Context) {
	w.log.Info("outbox worker started", "consumers", w.ConsumerNames(), "poll_interval", w.cfg.PollInterval.String(),
		"retention", w.cfg.Retention.String())
	wake := make(chan struct{}, 1)
	go w.listen(ctx, wake)
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()
	for {
		w.Drain(ctx)
		w.purgeIfDue(ctx)
		select {
		case <-ctx.Done():
			w.log.Info("outbox worker stopped")
			return
		case <-wake:
		case <-ticker.C:
		}
	}
}

// Drain delivers the due events batch after batch until none is left.
func (w *OutboxWorker) Drain(ctx context.Context) {
	for ctx.Err() == nil {
		n, err := w.ProcessBatch(ctx)
		if err != nil {
			if ctx.Err() == nil {
				w.log.Error("outbox worker", "error", err)
			}
			return
		}
		if n == 0 {
			return
		}
	}
}

// ProcessBatch claims a batch of due events and delivers each to every
// consumer; it returns how many events it claimed. An event every consumer
// accepted is published; otherwise it is retried later, or given up after
// OutboxMaxAttempts.
func (w *OutboxWorker) ProcessBatch(ctx context.Context) (int, error) {
	args := pgx.NamedArgs{"max_attempts": OutboxMaxAttempts, "lease": w.cfg.Lease, "batch": w.cfg.Batch}
	if _, err := w.pool.Exec(ctx, failExhaustedOutboxSQL, args); err != nil {
		return 0, fmt.Errorf("give up interrupted outbox deliveries: %w", err)
	}
	rows, err := w.pool.Query(ctx, claimOutboxEventsSQL, args)
	if err != nil {
		return 0, fmt.Errorf("claim outbox events: %w", err)
	}
	events, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[OutboxEvent])
	if err != nil {
		return 0, fmt.Errorf("read outbox events: %w", err)
	}
	var published []uuid.UUID
	for i, ev := range events {
		failed, msg := w.deliver(ctx, ev)
		if ctx.Err() != nil {
			// Shutting down: the rest goes back to the queue, uncounted.
			return len(events), w.requeue(ctx, events[i:], published)
		}
		if len(failed) == 0 {
			published = append(published, ev.ID)
			continue
		}
		if err := w.retry(ctx, ev, failed, msg); err != nil {
			return len(events), err
		}
	}
	if err := w.publish(ctx, published); err != nil {
		return len(events), err
	}
	return len(events), nil
}

// deliver hands ev to every consumer and returns those that failed, with
// their errors joined; a panicking consumer counts as failed.
func (w *OutboxWorker) deliver(ctx context.Context, ev *OutboxEvent) ([]string, string) {
	var failed, msgs []string
	for _, c := range w.consumers {
		if err := handleSafely(ctx, c, ev); err != nil {
			failed = append(failed, c.Name())
			msgs = append(msgs, c.Name()+": "+err.Error())
		}
	}
	msg := strings.Join(msgs, "; ")
	if len(msg) > maxOutboxError {
		msg = msg[:maxOutboxError]
	}
	return failed, msg
}

// handleSafely calls the consumer, turning a panic into an error so one
// faulty consumer cannot stop the server.
func handleSafely(ctx context.Context, c OutboxConsumer, ev *OutboxEvent) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return c.Handle(ctx, ev)
}

// publish records the events every consumer accepted.
func (w *OutboxWorker) publish(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	if _, err := w.pool.Exec(ctx, publishOutboxEventsSQL, pgx.NamedArgs{"ids": ids}); err != nil {
		return fmt.Errorf("publish outbox events: %w", err)
	}
	return nil
}

// retry records a failed delivery of ev: tried again after a growing delay,
// or given up after OutboxMaxAttempts.
func (w *OutboxWorker) retry(ctx context.Context, ev *OutboxEvent, failed []string, msg string) error {
	w.log.Warn("outbox delivery failed", "event_id", ev.ID, "event_type", ev.EventType, "attempt", ev.Attempts,
		"consumers", failed, "error", msg)
	_, err := w.pool.Exec(ctx, retryOutboxEventSQL, pgx.NamedArgs{
		"id": ev.ID, "last_error": msg, "failed_consumers": failed,
		"delay": OutboxRetryDelay(ev.Attempts), "max_attempts": OutboxMaxAttempts,
	})
	if err != nil {
		return fmt.Errorf("record failed outbox delivery %s: %w", ev.ID, err)
	}
	return nil
}

// requeue publishes what was delivered before a shutdown and releases the
// claims of the rest, outliving the cancelled ctx for a few seconds.
func (w *OutboxWorker) requeue(ctx context.Context, rest []*OutboxEvent, published []uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), outboxRequeueTimeout)
	defer cancel()
	if err := w.publish(ctx, published); err != nil {
		return err
	}
	ids := make([]uuid.UUID, len(rest))
	for i, ev := range rest {
		ids[i] = ev.ID
	}
	if _, err := w.pool.Exec(ctx, requeueOutboxEventsSQL, pgx.NamedArgs{"ids": ids}); err != nil {
		return fmt.Errorf("release outbox claims: %w", err)
	}
	return nil
}

// purgeIfDue deletes the delivered events past the retention, at most every
// outboxPurgeEvery.
func (w *OutboxWorker) purgeIfDue(ctx context.Context) {
	if ctx.Err() != nil || time.Since(w.lastPurge) < outboxPurgeEvery {
		return
	}
	w.lastPurge = time.Now()
	if n, err := w.Purge(ctx); err != nil {
		w.log.Error("outbox purge", "error", err)
	} else if n > 0 {
		w.log.Info("outbox purged", "events", n)
	}
}

// Purge deletes the events delivered before the retention, in batches, and
// returns how many it deleted.
func (w *OutboxWorker) Purge(ctx context.Context) (int64, error) {
	var total int64
	for {
		tag, err := w.pool.Exec(ctx, purgeOutboxEventsSQL, pgx.NamedArgs{"retention": w.cfg.Retention, "batch": outboxPurgeBatch})
		if err != nil {
			return total, fmt.Errorf("purge outbox events: %w", err)
		}
		total += tag.RowsAffected()
		if tag.RowsAffected() < outboxPurgeBatch {
			return total, nil
		}
	}
}

// listen wakes the worker on every notification of OutboxChannel, on a
// dedicated connection (outside the pool, which it would otherwise hold for
// good); it reconnects after an error, the poll covering the gap.
func (w *OutboxWorker) listen(ctx context.Context, wake chan<- struct{}) {
	for ctx.Err() == nil {
		err := w.listenOnce(ctx, wake)
		if ctx.Err() != nil {
			return
		}
		w.log.Warn("outbox notifications interrupted, polling meanwhile", "error", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(w.cfg.PollInterval):
		}
	}
}

// listenOnce opens the connection, listens and forwards notifications until
// an error or the end of ctx.
func (w *OutboxWorker) listenOnce(ctx context.Context, wake chan<- struct{}) error {
	conn, err := pgx.ConnectConfig(ctx, w.pool.Config().ConnConfig.Copy())
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	if _, err := conn.Exec(ctx, "LISTEN "+OutboxChannel); err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	for {
		if _, err := conn.WaitForNotification(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		select {
		case wake <- struct{}{}:
		default: // a wake-up is already pending
		}
	}
}
