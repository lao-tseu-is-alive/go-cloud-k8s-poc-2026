package retention

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// Transaction helpers called by casefile inside its transition transaction.

// StartCaseRetentionTx computes the retention of a case just closed in the
// transaction of q, from its type's default policy, and audits it on the case
// (RETENTION_STARTED). It returns nil when the type has no policy.
func StartCaseRetentionTx(ctx context.Context, q core.Querier, caseID uuid.UUID, operatorID, reason string) (*Retention, error) {
	r, err := collectRetention(q.Query(ctx, startRetentionSQL, pgx.NamedArgs{"case_id": caseID, "operator_id": operatorID}))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("start retention: %w", err)
	}
	if _, err := core.InsertAuditEventTx(ctx, q, core.AuditEvent{
		SubjectID: caseID, EventType: EventRetentionStarted, ActorUserID: operatorID, Reason: reason,
		AfterState: r.State(),
	}); err != nil {
		return nil, fmt.Errorf("audit retention start: %w", err)
	}
	return r, nil
}

// EndCaseRetentionTx ends the running retention of a case being reopened, in
// the transaction of q, and audits it on the case (RETENTION_ENDED). It
// returns nil when the case had none.
func EndCaseRetentionTx(ctx context.Context, q core.Querier, caseID uuid.UUID, operatorID, reason string) (*Retention, error) {
	r, err := collectRetention(q.Query(ctx, endRetentionSQL, pgx.NamedArgs{"subject_id": caseID, "operator_id": operatorID, "reason": reason}))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("end retention: %w", err)
	}
	if _, err := core.InsertAuditEventTx(ctx, q, core.AuditEvent{
		SubjectID: caseID, EventType: EventRetentionEnded, ActorUserID: operatorID, Reason: reason,
		BeforeState: r.State(),
	}); err != nil {
		return nil, fmt.Errorf("audit retention end: %w", err)
	}
	return r, nil
}

func collectRetention(rows pgx.Rows, err error) (*Retention, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Retention])
}

func collectHold(rows pgx.Rows, err error) (*Hold, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Hold])
}

// EnsureActivePolicyTx checks that code names an active policy, before a case
// type is given it (an empty code, meaning none, passes).
func EnsureActivePolicyTx(ctx context.Context, q core.Querier, code string) error {
	if code == "" {
		return nil
	}
	var active bool
	err := q.QueryRow(ctx, policyActiveSQL, pgx.NamedArgs{"code": code}).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: unknown retention policy %q", core.ErrInvalidInput, code)
	}
	if err != nil {
		return fmt.Errorf("read retention policy: %w", err)
	}
	if !active {
		return fmt.Errorf("%w: retention policy %q is inactive", core.ErrInvalidInput, code)
	}
	return nil
}
