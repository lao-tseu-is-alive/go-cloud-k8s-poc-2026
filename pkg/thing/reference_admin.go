package thing

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// ThingTypeInput is a new generic thing type (administrators only, GLD-040);
// parcel and building types are seeded.
type ThingTypeInput struct {
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

// ThingTypeUpdate changes a thing type; nil fields are kept, and the code and
// specialization are immutable.
type ThingTypeUpdate struct {
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

// thingTypeState is the logged state of a thing type.
func thingTypeState(e *ThingType) map[string]any {
	return map[string]any{"label": e.Label, "description": e.Description, "specialization": int16(e.Specialization), "is_active": e.IsActive}
}

// CreateThingType validates and adds a generic thing type.
func (s *Service) CreateThingType(ctx context.Context, in ThingTypeInput) (*ThingType, *core.ReferenceChange, error) {
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
	return s.repo.CreateThingType(ctx, in)
}

// UpdateThingType validates and applies a thing type change.
func (s *Service) UpdateThingType(ctx context.Context, code string, in ThingTypeUpdate) (*ThingType, *core.ReferenceChange, error) {
	if err := core.NormalizeOptionalReferenceText("label", &in.Label, core.MaxReferenceLabelLength, true); err != nil {
		return nil, nil, err
	}
	if err := core.NormalizeOptionalReferenceText("description", &in.Description, core.MaxReferenceDescriptionLength, false); err != nil {
		return nil, nil, err
	}
	return s.repo.UpdateThingType(ctx, strings.TrimSpace(code), in)
}

// CreateThingType inserts the type and its REFERENCE_CREATED log entry.
func (r *PostgresRepository) CreateThingType(ctx context.Context, in ThingTypeInput) (*ThingType, *core.ReferenceChange, error) {
	return core.MutateReference(ctx, r.pool, core.ReferenceMutation[ThingType]{
		Catalogue: core.CatalogueThingType, Code: in.Code, OperatorID: in.OperatorID, Reason: in.Reason,
		State: thingTypeState,
		Apply: func(ctx context.Context, tx pgx.Tx) (*ThingType, *ThingType, error) {
			rows, err := tx.Query(ctx, insertThingTypeSQL, pgx.NamedArgs{"code": in.Code, "label": in.Label, "description": in.Description})
			if err != nil {
				return nil, nil, core.MapReferenceConflict(err, core.CatalogueThingType, in.Code)
			}
			created, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[ThingType])
			return nil, created, core.MapReferenceConflict(err, core.CatalogueThingType, in.Code)
		},
	})
}

// UpdateThingType locks the type, applies the change and logs it.
func (r *PostgresRepository) UpdateThingType(ctx context.Context, code string, in ThingTypeUpdate) (*ThingType, *core.ReferenceChange, error) {
	return core.MutateReference(ctx, r.pool, core.ReferenceMutation[ThingType]{
		Catalogue: core.CatalogueThingType, Code: code, OperatorID: in.OperatorID, Reason: in.Reason,
		State: thingTypeState,
		Apply: func(ctx context.Context, tx pgx.Tx) (*ThingType, *ThingType, error) {
			before, err := core.CollectReferenceRow[ThingType](tx.Query(ctx, getThingTypeForUpdateSQL, pgx.NamedArgs{"code": code}))
			if err != nil {
				return nil, nil, fmt.Errorf("thing type %q: %w", code, err)
			}
			after, err := core.CollectReferenceRow[ThingType](tx.Query(ctx, updateThingTypeSQL, pgx.NamedArgs{
				"code": code, "label": in.Label, "description": in.Description, "is_active": in.IsActive,
			}))
			return before, after, err
		},
	})
}
