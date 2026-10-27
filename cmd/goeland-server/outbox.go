package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// buildOutboxWorker builds the outbox worker (GLD-028) with the consumers the
// configuration enables; the search indexer and the notifiers will join here.
func buildOutboxWorker(config serverConfig, pool *pgxpool.Pool, log *slog.Logger) (*core.OutboxWorker, error) {
	var consumers []core.OutboxConsumer
	if config.OutboxLog {
		consumers = append(consumers, logConsumer{log: log})
	}
	worker, err := core.NewOutboxWorker(pool, core.OutboxConfig{
		Retention: config.OutboxRetention, PollInterval: config.OutboxPollInterval,
	}, log, consumers...)
	if err != nil {
		return nil, fmt.Errorf("outbox worker: %w", err)
	}
	return worker, nil
}

// logConsumer logs every outbox event (GOELAND_OUTBOX_LOG=true): a way to
// watch the outbox work, references only, never business data.
type logConsumer struct {
	log *slog.Logger
}

// Name identifies the consumer.
func (logConsumer) Name() string { return "log" }

// Handle logs the event.
func (c logConsumer) Handle(ctx context.Context, ev *core.OutboxEvent) error {
	c.log.InfoContext(ctx, "outbox event", "event_id", ev.ID, "event_type", ev.EventType,
		"subject_kind", ev.SubjectKind, "subject_id", ev.SubjectID, "attempt", ev.Attempts)
	return nil
}
