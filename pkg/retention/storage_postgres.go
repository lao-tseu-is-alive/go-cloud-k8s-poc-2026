package retention

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// PostgresRepository implements Repository with pgx, composing core primitives.
type PostgresRepository struct {
	pool *pgxpool.Pool
	log  *slog.Logger
	// store holds the document bytes an elimination destroys; nil refuses eliminations.
	store blobstore.Store
	// disposalEnabled lets an elimination run; off, it is only simulated.
	disposalEnabled bool
}

// WithBlobStore sets the store whose bytes an elimination destroys (GLD-077).
func (r *PostgresRepository) WithBlobStore(store blobstore.Store) *PostgresRepository {
	r.store = store
	return r
}

// EnableDisposal lets the execution of elimination bordereaux run. It is off
// by default: an elimination is then only simulated (SimulateElimination), so
// destroying records is an explicit decision of the deployment.
func (r *PostgresRepository) EnableDisposal(enabled bool) *PostgresRepository {
	r.disposalEnabled = enabled
	return r
}

// NewPostgresRepository builds a PostgresRepository from a connection pool. A
// nil logger falls back to slog.Default.
func NewPostgresRepository(pool *pgxpool.Pool, log *slog.Logger) (*PostgresRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("%w: PostgreSQL pool is required", core.ErrInvalidInput)
	}
	if log == nil {
		log = slog.Default()
	}
	return &PostgresRepository{pool: pool, log: log}, nil
}

// policyState is the logged state of a policy.
func policyState(p *Policy) map[string]any {
	return map[string]any{
		"label": p.Label, "description": p.Description, "legal_basis": p.LegalBasis,
		"duration_years": p.DurationYears, "final_disposition": string(p.FinalDisposition), "is_active": p.IsActive,
	}
}

// ListPolicies returns the policies by label.
func (r *PostgresRepository) ListPolicies(ctx context.Context, onlyActive bool) ([]*Policy, error) {
	rows, err := r.pool.Query(ctx, listPoliciesSQL, pgx.NamedArgs{"only_active": onlyActive})
	if err != nil {
		return nil, fmt.Errorf("list retention policies: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[Policy])
}

// CreatePolicy inserts the policy and its REFERENCE_CREATED log entry.
func (r *PostgresRepository) CreatePolicy(ctx context.Context, in PolicyInput) (*Policy, *core.ReferenceChange, error) {
	return core.MutateReference(ctx, r.pool, core.ReferenceMutation[Policy]{
		Catalogue: core.CatalogueRetentionPolicy, Code: in.Code, OperatorID: in.OperatorID, Reason: in.Reason,
		State: policyState,
		Apply: func(ctx context.Context, tx pgx.Tx) (*Policy, *Policy, error) {
			rows, err := tx.Query(ctx, insertPolicySQL, pgx.NamedArgs{
				"code": in.Code, "label": in.Label, "description": in.Description, "legal_basis": in.LegalBasis,
				"duration_years": in.DurationYears, "final_disposition": string(in.FinalDisposition),
			})
			if err != nil {
				return nil, nil, core.MapReferenceConflict(err, core.CatalogueRetentionPolicy, in.Code)
			}
			created, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Policy])
			return nil, created, core.MapReferenceConflict(err, core.CatalogueRetentionPolicy, in.Code)
		},
	})
}

// UpdatePolicy locks the policy, applies the change and logs it.
func (r *PostgresRepository) UpdatePolicy(ctx context.Context, code string, in PolicyUpdate) (*Policy, *core.ReferenceChange, error) {
	var disposition *string
	if in.FinalDisposition != nil {
		d := string(*in.FinalDisposition)
		disposition = &d
	}
	return core.MutateReference(ctx, r.pool, core.ReferenceMutation[Policy]{
		Catalogue: core.CatalogueRetentionPolicy, Code: code, OperatorID: in.OperatorID, Reason: in.Reason,
		State: policyState,
		Apply: func(ctx context.Context, tx pgx.Tx) (*Policy, *Policy, error) {
			before, err := core.CollectReferenceRow[Policy](tx.Query(ctx, getPolicyForUpdateSQL, pgx.NamedArgs{"code": code}))
			if err != nil {
				return nil, nil, fmt.Errorf("retention policy %q: %w", code, err)
			}
			after, err := core.CollectReferenceRow[Policy](tx.Query(ctx, updatePolicySQL, pgx.NamedArgs{
				"code": code, "label": in.Label, "description": in.Description, "legal_basis": in.LegalBasis,
				"duration_years": in.DurationYears, "final_disposition": disposition, "is_active": in.IsActive,
			}))
			return before, after, err
		},
	})
}

// CaseRetention reads the retentions, holds and default policy of a case.
func (r *PostgresRepository) CaseRetention(ctx context.Context, caseID uuid.UUID) (*CaseRetention, error) {
	if err := findCase(ctx, r.pool, liveCaseSQL, caseID); err != nil {
		return nil, err
	}
	var out CaseRetention
	rows, err := r.pool.Query(ctx, listRetentionsSQL, pgx.NamedArgs{"subject_id": caseID})
	if err != nil {
		return nil, fmt.Errorf("list retentions: %w", err)
	}
	if out.Retentions, err = pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[Retention]); err != nil {
		return nil, fmt.Errorf("list retentions: %w", err)
	}
	if rows, err = r.pool.Query(ctx, listHoldsSQL, pgx.NamedArgs{"subject_id": caseID}); err != nil {
		return nil, fmt.Errorf("list holds: %w", err)
	}
	if out.Holds, err = pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[Hold]); err != nil {
		return nil, fmt.Errorf("list holds: %w", err)
	}
	policy, err := core.CollectReferenceRow[Policy](r.pool.Query(ctx, casePolicySQL, pgx.NamedArgs{"case_id": caseID}))
	if err != nil && !errors.Is(err, core.ErrNotFound) {
		return nil, fmt.Errorf("read case policy: %w", err)
	}
	out.DefaultPolicy = policy
	if len(out.Retentions) > 0 && out.Retentions[0].EndedAt == nil {
		out.Current = out.Retentions[0]
	}
	if len(out.Holds) > 0 && out.Holds[0].ReleasedAt == nil {
		out.Hold = out.Holds[0]
	}
	return &out, nil
}

// PlaceHold places the hold of a live case and writes RETENTION_HOLD_PLACED;
// a second active hold is a conflict.
func (r *PostgresRepository) PlaceHold(ctx context.Context, in HoldInput) (*Hold, *core.AuditEvent, error) {
	return r.changeHold(ctx, in, placeHoldSQL, EventHoldPlaced, fmt.Errorf("%w: the case is already on legal hold", core.ErrConflict))
}

// ReleaseHold releases the active hold of a live case and writes
// RETENTION_HOLD_RELEASED; no active hold is a precondition failure.
func (r *PostgresRepository) ReleaseHold(ctx context.Context, in HoldInput) (*Hold, *core.AuditEvent, error) {
	return r.changeHold(ctx, in, releaseHoldSQL, EventHoldReleased, nil)
}

// changeHold runs one hold change on a locked live case with its audit event;
// conflict answers a second active hold (nil when the change cannot create one).
func (r *PostgresRepository) changeHold(ctx context.Context, in HoldInput, sql, eventType string, conflict error) (*Hold, *core.AuditEvent, error) {
	var hold *Hold
	var ev *core.AuditEvent
	err := core.InTx(ctx, r.pool, "change retention hold", func(tx pgx.Tx) error {
		if err := findCase(ctx, tx, lockCaseSQL, in.CaseID); err != nil {
			return err
		}
		var err error
		hold, err = collectHold(tx.Query(ctx, sql, pgx.NamedArgs{"subject_id": in.CaseID, "reason": in.Reason, "operator_id": in.OperatorID}))
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: the case is not on legal hold", core.ErrInvalidState)
		}
		if _, unique := core.PgErrorWithCode(err, core.PgUniqueViolation); unique && conflict != nil {
			return conflict
		}
		if err != nil {
			return fmt.Errorf("change hold: %w", err)
		}
		ev, err = core.InsertAuditEventTx(ctx, tx, core.AuditEvent{
			SubjectID: in.CaseID, EventType: eventType, ActorUserID: in.OperatorID, Reason: in.Reason,
			AfterState: map[string]any{"hold_id": hold.ID, "on_hold": hold.ReleasedAt == nil},
		})
		return err
	})
	return hold, ev, err
}

// findCase runs sql (liveCaseSQL or lockCaseSQL) for a case, answering
// NotFound when it is missing, deleted or not a case.
func findCase(ctx context.Context, q core.Querier, sql string, caseID uuid.UUID) error {
	var id uuid.UUID
	err := q.QueryRow(ctx, sql, pgx.NamedArgs{"case_id": caseID}).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: case %s", core.ErrNotFound, caseID)
	}
	return err
}

// ListDue pages the due retentions.
func (r *PostgresRepository) ListDue(ctx context.Context, filter DueFilter) (DueResult, error) {
	rows, err := r.pool.Query(ctx, dueRetentionsSQL, pgx.NamedArgs{
		"due_on": filter.DueOn, "disposition": string(filter.Disposition), "include_held": filter.IncludeHeld,
		"limit": filter.Limit, "offset": filter.Offset,
	})
	if err != nil {
		return DueResult{}, fmt.Errorf("list due retentions: %w", err)
	}
	due, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[DueRetention])
	if err != nil {
		return DueResult{}, fmt.Errorf("list due retentions: %w", err)
	}
	res := DueResult{Retentions: due}
	if len(due) > 0 {
		res.TotalSize = due[0].TotalSize
	}
	return res, nil
}

// ApplyCaseType gives the type's policy to at most MaxApplyBatch closed cases
// without retention, each audited, in one transaction.
func (r *PostgresRepository) ApplyCaseType(ctx context.Context, in ApplyInput) (ApplyResult, error) {
	var res ApplyResult
	err := core.InTx(ctx, r.pool, "apply case type retention", func(tx pgx.Tx) error {
		var policy string
		err := tx.QueryRow(ctx, caseTypePolicySQL, pgx.NamedArgs{"code": in.CaseTypeCode}).Scan(&policy)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: case type %q", core.ErrNotFound, in.CaseTypeCode)
		}
		if err != nil {
			return fmt.Errorf("read case type policy: %w", err)
		}
		if policy == "" {
			return fmt.Errorf("%w: case type %q has no retention policy", core.ErrInvalidState, in.CaseTypeCode)
		}
		ids, err := pendingCasesTx(ctx, tx, in.CaseTypeCode)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if _, err := StartCaseRetentionTx(ctx, tx, id, in.OperatorID, in.Reason); err != nil {
				return err
			}
		}
		res.Applied = int32(len(ids))
		return tx.QueryRow(ctx, countPendingApplySQL, pgx.NamedArgs{"case_type_code": in.CaseTypeCode}).Scan(&res.Remaining)
	})
	return res, err
}

// pendingCasesTx lists the next closed cases of a type without retention.
func pendingCasesTx(ctx context.Context, tx pgx.Tx, caseTypeCode string) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, pendingApplySQL, pgx.NamedArgs{"case_type_code": caseTypeCode, "limit": MaxApplyBatch})
	if err != nil {
		return nil, fmt.Errorf("list closed cases without retention: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return nil, fmt.Errorf("list closed cases without retention: %w", err)
	}
	return ids, nil
}
