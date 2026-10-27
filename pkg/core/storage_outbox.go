package core

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// OutboxStatus counts the outbox events by state, the given-up events per
// consumer and lists the latest given-up events (GLD-028).
func (r *PostgresRepository) OutboxStatus(ctx context.Context) (OutboxStatus, error) {
	rows, err := r.pool.Query(ctx, outboxCountsSQL)
	if err != nil {
		return OutboxStatus{}, fmt.Errorf("count outbox events: %w", err)
	}
	status, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[OutboxStatus])
	if err != nil {
		return OutboxStatus{}, fmt.Errorf("read outbox counts: %w", err)
	}
	if rows, err = r.pool.Query(ctx, outboxFailedByConsumerSQL); err != nil {
		return OutboxStatus{}, fmt.Errorf("count failed outbox events: %w", err)
	}
	if status.Consumers, err = pgx.CollectRows(rows, pgx.RowToStructByNameLax[OutboxConsumerStatus]); err != nil {
		return OutboxStatus{}, fmt.Errorf("read failed outbox counts: %w", err)
	}
	if rows, err = r.pool.Query(ctx, recentFailedOutboxSQL, pgx.NamedArgs{"limit": recentFailedLimit}); err != nil {
		return OutboxStatus{}, fmt.Errorf("list failed outbox events: %w", err)
	}
	if status.RecentFailed, err = pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[OutboxEvent]); err != nil {
		return OutboxStatus{}, fmt.Errorf("read failed outbox events: %w", err)
	}
	return status, nil
}

// ReplayOutbox puts failed events back in the queue, those of ids or all of
// them, and returns how many.
func (r *PostgresRepository) ReplayOutbox(ctx context.Context, ids []uuid.UUID, all bool) (int32, error) {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	var replayed int32
	var notified bool
	if err := r.pool.QueryRow(ctx, replayOutboxEventsSQL, pgx.NamedArgs{"ids": ids, "all": all}).Scan(&replayed, &notified); err != nil {
		return 0, fmt.Errorf("replay outbox events: %w", err)
	}
	return replayed, nil
}
