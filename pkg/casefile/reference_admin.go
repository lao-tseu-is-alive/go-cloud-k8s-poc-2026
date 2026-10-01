package casefile

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
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
	// DefaultConfidentialityLevel is the minimum confidentiality of its new cases (0-5).
	DefaultConfidentialityLevel int32
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
	// DefaultConfidentialityLevel replaces the minimum confidentiality of future cases when non-nil.
	DefaultConfidentialityLevel *int32
	// OperatorID is the administrator, set server-side.
	OperatorID string
	// Reason is the justification recorded in the reference change log.
	Reason string
}

// caseTypeState is the logged state of a case type.
// The default grants are logged when loaded (SetCaseTypeDefaultGrants).
func caseTypeState(e *CaseType) map[string]any {
	state := map[string]any{
		"label": e.Label, "description": e.Description, "business_ref_namespace": e.BusinessRefNamespace,
		"is_active": e.IsActive, "default_confidentiality_level": e.DefaultConfidentialityLevel,
	}
	if e.DefaultGrants != nil {
		grants := make([]map[string]any, len(e.DefaultGrants))
		for i, g := range e.DefaultGrants {
			grants[i] = g.State()
		}
		state["default_grants"] = grants
	}
	return state
}

// validConfidentiality checks a default confidentiality level (0-5).
func validConfidentiality(level int32) error {
	if level < 0 || level > 5 {
		return fmt.Errorf("%w: default_confidentiality_level must be between 0 and 5", core.ErrInvalidInput)
	}
	return nil
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
	if err = validConfidentiality(in.DefaultConfidentialityLevel); err != nil {
		return nil, nil, err
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
	if in.BusinessRefNamespace != nil && !core.ValidBusinessRefNamespace(*in.BusinessRefNamespace) {
		return nil, nil, fmt.Errorf("%w: business_ref_namespace must be upper-case letters, digits and underscores, starting with a letter", core.ErrInvalidInput)
	}
	if in.DefaultConfidentialityLevel != nil {
		if err := validConfidentiality(*in.DefaultConfidentialityLevel); err != nil {
			return nil, nil, err
		}
	}
	return s.repo.UpdateCaseType(ctx, strings.TrimSpace(code), in)
}

// CreateCaseType inserts the entry and its REFERENCE_CREATED log entry.
func (r *PostgresRepository) CreateCaseType(ctx context.Context, in CaseTypeInput) (*CaseType, *core.ReferenceChange, error) {
	entry, change, err := core.MutateReference(ctx, r.pool, core.ReferenceMutation[CaseType]{
		Catalogue: core.CatalogueCaseType, Code: in.Code, OperatorID: in.OperatorID, Reason: in.Reason,
		State: caseTypeState,
		Apply: func(ctx context.Context, tx pgx.Tx) (*CaseType, *CaseType, error) {
			rows, err := tx.Query(ctx, insertCaseTypeSQL, pgx.NamedArgs{
				"code": in.Code, "label": in.Label, "description": in.Description, "business_ref_namespace": in.BusinessRefNamespace,
				"default_confidentiality_level": in.DefaultConfidentialityLevel,
			})
			if err != nil {
				return nil, nil, core.MapReferenceConflict(err, core.CatalogueCaseType, in.Code)
			}
			created, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[CaseType])
			return nil, created, core.MapReferenceConflict(err, core.CatalogueCaseType, in.Code)
		},
	})
	if err != nil {
		return nil, nil, err
	}
	return entry, change, hydrateDefaultGrantsTx(ctx, r.pool, []*CaseType{entry})
}

// UpdateCaseType locks the entry, applies the change and logs it.
func (r *PostgresRepository) UpdateCaseType(ctx context.Context, code string, in CaseTypeUpdate) (*CaseType, *core.ReferenceChange, error) {
	entry, change, err := core.MutateReference(ctx, r.pool, core.ReferenceMutation[CaseType]{
		Catalogue: core.CatalogueCaseType, Code: code, OperatorID: in.OperatorID, Reason: in.Reason,
		State: caseTypeState,
		Apply: func(ctx context.Context, tx pgx.Tx) (*CaseType, *CaseType, error) {
			before, err := core.CollectReferenceRow[CaseType](tx.Query(ctx, getCaseTypeForUpdateSQL, pgx.NamedArgs{"code": code}))
			if err != nil {
				return nil, nil, fmt.Errorf("case type %q: %w", code, err)
			}
			after, err := core.CollectReferenceRow[CaseType](tx.Query(ctx, updateCaseTypeSQL, pgx.NamedArgs{"code": code, "label": in.Label, "description": in.Description, "business_ref_namespace": in.BusinessRefNamespace, "is_active": in.IsActive,
				"default_confidentiality_level": in.DefaultConfidentialityLevel,
			}))
			return before, after, err
		},
	})
	if err != nil {
		return nil, nil, err
	}
	return entry, change, hydrateDefaultGrantsTx(ctx, r.pool, []*CaseType{entry})
}

// DefaultGrantsInput replaces the default grants of a case type (GLD-050).
type DefaultGrantsInput struct {
	// Grants is the whole new template; empty clears it.
	Grants []core.DefaultGrant
	// OperatorID is the administrator, set server-side.
	OperatorID string
	// Reason is the justification recorded in the reference change log.
	Reason string
}

// SetDefaultGrants replaces the default grants copied onto new cases of a
// type; existing cases keep theirs.
func (s *Service) SetDefaultGrants(ctx context.Context, code string, in DefaultGrantsInput) (*CaseType, *core.ReferenceChange, error) {
	if len(in.Grants) > core.MaxDefaultGrants {
		return nil, nil, fmt.Errorf("%w: at most %d default grants", core.ErrInvalidInput, core.MaxDefaultGrants)
	}
	return s.repo.SetDefaultGrants(ctx, strings.TrimSpace(code), in)
}

// SetDefaultGrants locks the case type, checks the grantees, replaces the
// template and logs the before and after templates.
func (r *PostgresRepository) SetDefaultGrants(ctx context.Context, code string, in DefaultGrantsInput) (*CaseType, *core.ReferenceChange, error) {
	return core.MutateReference(ctx, r.pool, core.ReferenceMutation[CaseType]{
		Catalogue: core.CatalogueCaseType, Code: code, OperatorID: in.OperatorID, Reason: in.Reason,
		State: caseTypeState,
		Apply: func(ctx context.Context, tx pgx.Tx) (*CaseType, *CaseType, error) {
			before, err := core.CollectReferenceRow[CaseType](tx.Query(ctx, getCaseTypeForUpdateSQL, pgx.NamedArgs{"code": code}))
			if err != nil {
				return nil, nil, fmt.Errorf("case type %q: %w", code, err)
			}
			if err := hydrateDefaultGrantsTx(ctx, tx, []*CaseType{before}); err != nil {
				return nil, nil, err
			}
			if err := core.ValidateDefaultGrantsTx(ctx, tx, in.Grants); err != nil {
				return nil, nil, err
			}
			if err := replaceDefaultGrantsTx(ctx, tx, before.ID, in); err != nil {
				return nil, nil, err
			}
			after := *before
			after.DefaultGrants = nil
			return before, &after, hydrateDefaultGrantsTx(ctx, tx, []*CaseType{&after})
		},
	})
}

// replaceDefaultGrantsTx deletes the template lines of a case type and writes
// the new ones (the history is the reference change log).
func replaceDefaultGrantsTx(ctx context.Context, tx pgx.Tx, caseTypeID uuid.UUID, in DefaultGrantsInput) error {
	if _, err := tx.Exec(ctx, deleteDefaultGrantsSQL, pgx.NamedArgs{"case_type_id": caseTypeID}); err != nil {
		return fmt.Errorf("clear default grants: %w", err)
	}
	for _, g := range in.Grants {
		if _, err := tx.Exec(ctx, insertDefaultGrantSQL, pgx.NamedArgs{
			"case_type_id": caseTypeID, "grantee_kind": string(g.GranteeKind), "grantee_user_id": g.GranteeUserID,
			"grantee_subject_id": g.GranteeSubjectID, "level": int16(g.Level), "created_by": in.OperatorID,
		}); err != nil {
			return fmt.Errorf("insert default grant: %w", err)
		}
	}
	return nil
}

// defaultGrantRow is a template line with the case type it belongs to.
type defaultGrantRow struct {
	// CaseTypeID is the case type of the line.
	CaseTypeID uuid.UUID `db:"case_type_id"`
	core.DefaultGrant
}

// hydrateDefaultGrantsTx loads the templates of types (an empty, non-nil slice without lines).
func hydrateDefaultGrantsTx(ctx context.Context, q core.Querier, types []*CaseType) error {
	if len(types) == 0 {
		return nil
	}
	grouped, err := core.CollectGroupedTx(ctx, q, listDefaultGrantsSQL,
		core.IDsOf(types, func(t *CaseType) uuid.UUID { return t.ID }), func(r *defaultGrantRow) uuid.UUID { return r.CaseTypeID })
	if err != nil {
		return fmt.Errorf("load default grants: %w", err)
	}
	for _, t := range types {
		t.DefaultGrants = make([]core.DefaultGrant, 0, len(grouped[t.ID]))
		for _, row := range grouped[t.ID] {
			t.DefaultGrants = append(t.DefaultGrants, row.DefaultGrant)
		}
	}
	return nil
}
