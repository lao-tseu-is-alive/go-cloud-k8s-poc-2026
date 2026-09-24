package actor

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// OrganizationCategoryInput is a new organization category (administrators only, GLD-040).
type OrganizationCategoryInput struct {
	// Code is the immutable key (see core.ValidateReferenceCode).
	Code string
	// Label is the required human label.
	Label string
	// OperatorID is the administrator, set server-side.
	OperatorID string
	// Reason is the justification recorded in the reference change log.
	Reason string
}

// OrganizationCategoryUpdate changes a organization category; nil fields are kept and the code is immutable.
type OrganizationCategoryUpdate struct {
	// Label replaces the label when non-nil (not blank).
	Label *string
	// IsActive activates or deactivates the entry when non-nil.
	IsActive *bool
	// OperatorID is the administrator, set server-side.
	OperatorID string
	// Reason is the justification recorded in the reference change log.
	Reason string
}

// organizationCategoryState is the logged state of a organization category.
func organizationCategoryState(e *OrganizationCategory) map[string]any {
	return map[string]any{"label": e.Label, "is_active": e.IsActive}
}

// CreateOrganizationCategory validates and adds a organization category.
func (s *Service) CreateOrganizationCategory(ctx context.Context, in OrganizationCategoryInput) (*OrganizationCategory, *core.ReferenceChange, error) {
	var err error
	in.Code = strings.TrimSpace(in.Code)
	if err = core.ValidateReferenceCode(in.Code); err != nil {
		return nil, nil, err
	}
	if in.Label, err = core.NormalizeReferenceText("label", in.Label, core.MaxReferenceLabelLength, true); err != nil {
		return nil, nil, err
	}
	return s.repo.CreateOrganizationCategory(ctx, in)
}

// UpdateOrganizationCategory validates and applies a organization category change.
func (s *Service) UpdateOrganizationCategory(ctx context.Context, code string, in OrganizationCategoryUpdate) (*OrganizationCategory, *core.ReferenceChange, error) {
	if err := core.NormalizeOptionalReferenceText("label", &in.Label, core.MaxReferenceLabelLength, true); err != nil {
		return nil, nil, err
	}
	return s.repo.UpdateOrganizationCategory(ctx, strings.TrimSpace(code), in)
}

// CreateOrganizationCategory inserts the entry and its REFERENCE_CREATED log entry.
func (r *PostgresRepository) CreateOrganizationCategory(ctx context.Context, in OrganizationCategoryInput) (*OrganizationCategory, *core.ReferenceChange, error) {
	return core.MutateReference(ctx, r.pool, core.ReferenceMutation[OrganizationCategory]{
		Catalogue: core.CatalogueOrganizationCategory, Code: in.Code, OperatorID: in.OperatorID, Reason: in.Reason,
		State: organizationCategoryState,
		Apply: func(ctx context.Context, tx pgx.Tx) (*OrganizationCategory, *OrganizationCategory, error) {
			rows, err := tx.Query(ctx, insertOrganizationCategorySQL, pgx.NamedArgs{"code": in.Code, "label": in.Label})
			if err != nil {
				return nil, nil, core.MapReferenceConflict(err, core.CatalogueOrganizationCategory, in.Code)
			}
			created, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[OrganizationCategory])
			return nil, created, core.MapReferenceConflict(err, core.CatalogueOrganizationCategory, in.Code)
		},
	})
}

// UpdateOrganizationCategory locks the entry, applies the change and logs it.
func (r *PostgresRepository) UpdateOrganizationCategory(ctx context.Context, code string, in OrganizationCategoryUpdate) (*OrganizationCategory, *core.ReferenceChange, error) {
	return core.MutateReference(ctx, r.pool, core.ReferenceMutation[OrganizationCategory]{
		Catalogue: core.CatalogueOrganizationCategory, Code: code, OperatorID: in.OperatorID, Reason: in.Reason,
		State: organizationCategoryState,
		Apply: func(ctx context.Context, tx pgx.Tx) (*OrganizationCategory, *OrganizationCategory, error) {
			before, err := core.CollectReferenceRow[OrganizationCategory](tx.Query(ctx, getOrganizationCategoryForUpdateSQL, pgx.NamedArgs{"code": code}))
			if err != nil {
				return nil, nil, fmt.Errorf("organization category %q: %w", code, err)
			}
			after, err := core.CollectReferenceRow[OrganizationCategory](tx.Query(ctx, updateOrganizationCategorySQL, pgx.NamedArgs{"code": code, "label": in.Label, "is_active": in.IsActive}))
			return before, after, err
		},
	})
}
