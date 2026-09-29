package orgunit

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// TypeInput is a new org unit type (administrators only, GLD-040); it is
// ordered after the seeded types.
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

// TypeUpdate changes an org unit type; nil fields are kept and the code is immutable.
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

// typeState is the logged state of an org unit type.
func typeState(e *OrgUnitType) map[string]any {
	return map[string]any{"label": e.Label, "description": e.Description, "sort_order": e.SortOrder, "is_active": e.IsActive}
}

// CreateType validates and adds an org unit type.
func (s *Service) CreateType(ctx context.Context, in TypeInput) (*OrgUnitType, *core.ReferenceChange, error) {
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

// UpdateType validates and applies an org unit type change.
func (s *Service) UpdateType(ctx context.Context, code string, in TypeUpdate) (*OrgUnitType, *core.ReferenceChange, error) {
	if err := core.NormalizeOptionalReferenceText("label", &in.Label, core.MaxReferenceLabelLength, true); err != nil {
		return nil, nil, err
	}
	if err := core.NormalizeOptionalReferenceText("description", &in.Description, core.MaxReferenceDescriptionLength, false); err != nil {
		return nil, nil, err
	}
	return s.repo.UpdateType(ctx, strings.TrimSpace(code), in)
}

// CreateType inserts the type and its REFERENCE_CREATED log entry.
func (r *PostgresRepository) CreateType(ctx context.Context, in TypeInput) (*OrgUnitType, *core.ReferenceChange, error) {
	return core.MutateReference(ctx, r.pool, core.ReferenceMutation[OrgUnitType]{
		Catalogue: core.CatalogueOrgUnitType, Code: in.Code, OperatorID: in.OperatorID, Reason: in.Reason,
		State: typeState,
		Apply: func(ctx context.Context, tx pgx.Tx) (*OrgUnitType, *OrgUnitType, error) {
			rows, err := tx.Query(ctx, insertTypeSQL, pgx.NamedArgs{"code": in.Code, "label": in.Label, "description": in.Description})
			if err != nil {
				return nil, nil, core.MapReferenceConflict(err, core.CatalogueOrgUnitType, in.Code)
			}
			created, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[OrgUnitType])
			return nil, created, core.MapReferenceConflict(err, core.CatalogueOrgUnitType, in.Code)
		},
	})
}

// UpdateType locks the type, applies the change and logs it.
func (r *PostgresRepository) UpdateType(ctx context.Context, code string, in TypeUpdate) (*OrgUnitType, *core.ReferenceChange, error) {
	return core.MutateReference(ctx, r.pool, core.ReferenceMutation[OrgUnitType]{
		Catalogue: core.CatalogueOrgUnitType, Code: code, OperatorID: in.OperatorID, Reason: in.Reason,
		State: typeState,
		Apply: func(ctx context.Context, tx pgx.Tx) (*OrgUnitType, *OrgUnitType, error) {
			before, err := core.CollectReferenceRow[OrgUnitType](tx.Query(ctx, getTypeForUpdateSQL, pgx.NamedArgs{"code": code}))
			if err != nil {
				return nil, nil, fmt.Errorf("org unit type %q: %w", code, err)
			}
			after, err := core.CollectReferenceRow[OrgUnitType](tx.Query(ctx, updateTypeSQL, pgx.NamedArgs{
				"code": code, "label": in.Label, "description": in.Description, "is_active": in.IsActive,
			}))
			return before, after, err
		},
	})
}
