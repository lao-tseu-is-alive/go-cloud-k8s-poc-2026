package task

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// TypeInput is a new task type (administrators only, GLD-040).
type TypeInput struct {
	// Code is the immutable key (see core.ValidateReferenceCode).
	Code string
	// Label is the required human label.
	Label string
	// Description documents the business meaning.
	Description string
	// OperatorID is the administrator, set server-side.
	OperatorID string
	// Reason is the justification recorded in the reference change log.
	Reason string
}

// TypeUpdate changes an task type; nil fields are kept and the code is immutable.
type TypeUpdate struct {
	// Label replaces the label when non-nil (not blank).
	Label *string
	// Description replaces the description when non-nil.
	Description *string
	// IsActive activates or deactivates the type when non-nil.
	IsActive *bool
	// OperatorID is the administrator, set server-side.
	OperatorID string
	// Reason is the justification recorded in the reference change log.
	Reason string
}

// typeState is the logged state of an task type.
func typeState(e *TaskType) map[string]any {
	return map[string]any{"label": e.Label, "description": e.Description, "is_active": e.IsActive}
}

// CreateType validates and adds an task type.
func (s *Service) CreateType(ctx context.Context, in TypeInput) (*TaskType, *core.ReferenceChange, error) {
	var err error
	in.Code = strings.TrimSpace(in.Code)
	if err = core.ValidateReferenceCode(in.Code); err != nil {
		return nil, nil, err
	}
	if in.Label, err = core.NormalizeReferenceText("label", in.Label, core.MaxReferenceLabelLength, true); err != nil {
		return nil, nil, err
	}
	if in.Description, err = core.NormalizeReferenceText("description", in.Description, core.MaxReferenceDescriptionLength, false); err != nil {
		return nil, nil, err
	}
	return s.repo.CreateType(ctx, in)
}

// UpdateType validates and applies an task type change.
func (s *Service) UpdateType(ctx context.Context, code string, in TypeUpdate) (*TaskType, *core.ReferenceChange, error) {
	if err := core.NormalizeOptionalReferenceText("label", &in.Label, core.MaxReferenceLabelLength, true); err != nil {
		return nil, nil, err
	}
	if err := core.NormalizeOptionalReferenceText("description", &in.Description, core.MaxReferenceDescriptionLength, false); err != nil {
		return nil, nil, err
	}
	return s.repo.UpdateType(ctx, strings.TrimSpace(code), in)
}

// CreateType inserts the type and its REFERENCE_CREATED log entry.
func (r *PostgresRepository) CreateType(ctx context.Context, in TypeInput) (*TaskType, *core.ReferenceChange, error) {
	return core.MutateReference(ctx, r.pool, core.ReferenceMutation[TaskType]{
		Catalogue: core.CatalogueTaskType, Code: in.Code, OperatorID: in.OperatorID, Reason: in.Reason,
		State: typeState,
		Apply: func(ctx context.Context, tx pgx.Tx) (*TaskType, *TaskType, error) {
			rows, err := tx.Query(ctx, insertTypeSQL, pgx.NamedArgs{"code": in.Code, "label": in.Label, "description": in.Description})
			if err != nil {
				return nil, nil, core.MapReferenceConflict(err, core.CatalogueTaskType, in.Code)
			}
			created, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[TaskType])
			return nil, created, core.MapReferenceConflict(err, core.CatalogueTaskType, in.Code)
		},
	})
}

// UpdateType locks the type, applies the change and logs it.
func (r *PostgresRepository) UpdateType(ctx context.Context, code string, in TypeUpdate) (*TaskType, *core.ReferenceChange, error) {
	return core.MutateReference(ctx, r.pool, core.ReferenceMutation[TaskType]{
		Catalogue: core.CatalogueTaskType, Code: code, OperatorID: in.OperatorID, Reason: in.Reason,
		State: typeState,
		Apply: func(ctx context.Context, tx pgx.Tx) (*TaskType, *TaskType, error) {
			before, err := core.CollectReferenceRow[TaskType](tx.Query(ctx, getTypeForUpdateSQL, pgx.NamedArgs{"code": code}))
			if err != nil {
				return nil, nil, fmt.Errorf("task type %q: %w", code, err)
			}
			after, err := core.CollectReferenceRow[TaskType](tx.Query(ctx, updateTypeSQL, pgx.NamedArgs{
				"code": code, "label": in.Label, "description": in.Description, "is_active": in.IsActive,
			}))
			return before, after, err
		},
	})
}
