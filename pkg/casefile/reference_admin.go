package casefile

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// CaseTypeInput is a new case type (administrators only, GLD-040).
type CaseTypeInput struct {
	// Code is the immutable key (see core.ValidateReferenceCode).
	Code string
	// Label is the required human label.
	Label string
	// Description documents the business meaning.
	Description string
	// BusinessRefNamespace is where cases of this type get their reference; empty allocates none.
	BusinessRefNamespace string
	// OperatorID is the administrator, set server-side.
	OperatorID string
	// Reason is the justification recorded in the reference change log.
	Reason string
}

// CaseTypeUpdate changes a case type; nil fields are kept and the code is immutable.
type CaseTypeUpdate struct {
	// Label replaces the label when non-nil (not blank).
	Label *string
	// Description replaces the description when non-nil.
	Description *string
	// BusinessRefNamespace replaces the business ref namespace when non-nil.
	BusinessRefNamespace *string
	// IsActive activates or deactivates the entry when non-nil.
	IsActive *bool
	// OperatorID is the administrator, set server-side.
	OperatorID string
	// Reason is the justification recorded in the reference change log.
	Reason string
}

// caseTypeState is the logged state of a case type.
func caseTypeState(e *CaseType) map[string]any {
	return map[string]any{"label": e.Label, "description": e.Description, "business_ref_namespace": e.BusinessRefNamespace, "is_active": e.IsActive}
}

// CreateCaseType validates and adds a case type.
func (s *Service) CreateCaseType(ctx context.Context, in CaseTypeInput) (*CaseType, *core.ReferenceChange, error) {
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
	if in.BusinessRefNamespace, err = core.NormalizeReferenceText("business_ref_namespace", in.BusinessRefNamespace, 32, false); err != nil {
		return nil, nil, err
	}
	if !core.ValidBusinessRefNamespace(in.BusinessRefNamespace) {
		return nil, nil, fmt.Errorf("%w: business_ref_namespace must be upper-case letters, digits and underscores, starting with a letter", core.ErrInvalidInput)
	}
	return s.repo.CreateCaseType(ctx, in)
}

// UpdateCaseType validates and applies a case type change.
func (s *Service) UpdateCaseType(ctx context.Context, code string, in CaseTypeUpdate) (*CaseType, *core.ReferenceChange, error) {
	if err := core.NormalizeOptionalReferenceText("label", &in.Label, core.MaxReferenceLabelLength, true); err != nil {
		return nil, nil, err
	}
	if err := core.NormalizeOptionalReferenceText("description", &in.Description, core.MaxReferenceDescriptionLength, false); err != nil {
		return nil, nil, err
	}
	if err := core.NormalizeOptionalReferenceText("business_ref_namespace", &in.BusinessRefNamespace, 32, false); err != nil {
		return nil, nil, err
	}
	if in.BusinessRefNamespace != nil {
		if !core.ValidBusinessRefNamespace(*in.BusinessRefNamespace) {
			return nil, nil, fmt.Errorf("%w: business_ref_namespace must be upper-case letters, digits and underscores, starting with a letter", core.ErrInvalidInput)
		}
	}
	return s.repo.UpdateCaseType(ctx, strings.TrimSpace(code), in)
}

// CreateCaseType inserts the entry and its REFERENCE_CREATED log entry.
func (r *PostgresRepository) CreateCaseType(ctx context.Context, in CaseTypeInput) (*CaseType, *core.ReferenceChange, error) {
	return core.MutateReference(ctx, r.pool, core.ReferenceMutation[CaseType]{
		Catalogue: core.CatalogueCaseType, Code: in.Code, OperatorID: in.OperatorID, Reason: in.Reason,
		State: caseTypeState,
		Apply: func(ctx context.Context, tx pgx.Tx) (*CaseType, *CaseType, error) {
			rows, err := tx.Query(ctx, insertCaseTypeSQL, pgx.NamedArgs{"code": in.Code, "label": in.Label, "description": in.Description, "business_ref_namespace": in.BusinessRefNamespace})
			if err != nil {
				return nil, nil, core.MapReferenceConflict(err, core.CatalogueCaseType, in.Code)
			}
			created, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[CaseType])
			return nil, created, core.MapReferenceConflict(err, core.CatalogueCaseType, in.Code)
		},
	})
}

// UpdateCaseType locks the entry, applies the change and logs it.
func (r *PostgresRepository) UpdateCaseType(ctx context.Context, code string, in CaseTypeUpdate) (*CaseType, *core.ReferenceChange, error) {
	return core.MutateReference(ctx, r.pool, core.ReferenceMutation[CaseType]{
		Catalogue: core.CatalogueCaseType, Code: code, OperatorID: in.OperatorID, Reason: in.Reason,
		State: caseTypeState,
		Apply: func(ctx context.Context, tx pgx.Tx) (*CaseType, *CaseType, error) {
			before, err := core.CollectReferenceRow[CaseType](tx.Query(ctx, getCaseTypeForUpdateSQL, pgx.NamedArgs{"code": code}))
			if err != nil {
				return nil, nil, fmt.Errorf("case type %q: %w", code, err)
			}
			after, err := core.CollectReferenceRow[CaseType](tx.Query(ctx, updateCaseTypeSQL, pgx.NamedArgs{"code": code, "label": in.Label, "description": in.Description, "business_ref_namespace": in.BusinessRefNamespace, "is_active": in.IsActive}))
			return before, after, err
		},
	})
}
