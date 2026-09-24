package document

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// DocumentTypeInput is a new document type (administrators only, GLD-040).
type DocumentTypeInput struct {
	// Code is the immutable key (see core.ValidateReferenceCode).
	Code string
	// Label is the required human label.
	Label string
	// Description documents the business meaning.
	Description string
	// Category groups the type, e.g. ENTREE, PLAN, DECISION.
	Category string
	// OperatorID is the administrator, set server-side.
	OperatorID string
	// Reason is the justification recorded in the reference change log.
	Reason string
}

// DocumentTypeUpdate changes a document type; nil fields are kept and the code is immutable.
type DocumentTypeUpdate struct {
	// Label replaces the label when non-nil (not blank).
	Label *string
	// Description replaces the description when non-nil.
	Description *string
	// Category replaces the category when non-nil.
	Category *string
	// IsActive activates or deactivates the entry when non-nil.
	IsActive *bool
	// OperatorID is the administrator, set server-side.
	OperatorID string
	// Reason is the justification recorded in the reference change log.
	Reason string
}

// documentTypeState is the logged state of a document type.
func documentTypeState(e *DocumentType) map[string]any {
	return map[string]any{"label": e.Label, "description": e.Description, "category": e.Category, "is_active": e.IsActive}
}

// CreateDocumentType validates and adds a document type.
func (s *Service) CreateDocumentType(ctx context.Context, in DocumentTypeInput) (*DocumentType, *core.ReferenceChange, error) {
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
	if in.Category, err = core.NormalizeReferenceText("category", in.Category, 100, false); err != nil {
		return nil, nil, err
	}
	return s.repo.CreateDocumentType(ctx, in)
}

// UpdateDocumentType validates and applies a document type change.
func (s *Service) UpdateDocumentType(ctx context.Context, code string, in DocumentTypeUpdate) (*DocumentType, *core.ReferenceChange, error) {
	if err := core.NormalizeOptionalReferenceText("label", &in.Label, core.MaxReferenceLabelLength, true); err != nil {
		return nil, nil, err
	}
	if err := core.NormalizeOptionalReferenceText("description", &in.Description, core.MaxReferenceDescriptionLength, false); err != nil {
		return nil, nil, err
	}
	if err := core.NormalizeOptionalReferenceText("category", &in.Category, 100, false); err != nil {
		return nil, nil, err
	}
	return s.repo.UpdateDocumentType(ctx, strings.TrimSpace(code), in)
}

// CreateDocumentType inserts the entry and its REFERENCE_CREATED log entry.
func (r *PostgresRepository) CreateDocumentType(ctx context.Context, in DocumentTypeInput) (*DocumentType, *core.ReferenceChange, error) {
	return core.MutateReference(ctx, r.pool, core.ReferenceMutation[DocumentType]{
		Catalogue: core.CatalogueDocumentType, Code: in.Code, OperatorID: in.OperatorID, Reason: in.Reason,
		State: documentTypeState,
		Apply: func(ctx context.Context, tx pgx.Tx) (*DocumentType, *DocumentType, error) {
			rows, err := tx.Query(ctx, insertDocumentTypeSQL, pgx.NamedArgs{"code": in.Code, "label": in.Label, "description": in.Description, "category": in.Category})
			if err != nil {
				return nil, nil, core.MapReferenceConflict(err, core.CatalogueDocumentType, in.Code)
			}
			created, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[DocumentType])
			return nil, created, core.MapReferenceConflict(err, core.CatalogueDocumentType, in.Code)
		},
	})
}

// UpdateDocumentType locks the entry, applies the change and logs it.
func (r *PostgresRepository) UpdateDocumentType(ctx context.Context, code string, in DocumentTypeUpdate) (*DocumentType, *core.ReferenceChange, error) {
	return core.MutateReference(ctx, r.pool, core.ReferenceMutation[DocumentType]{
		Catalogue: core.CatalogueDocumentType, Code: code, OperatorID: in.OperatorID, Reason: in.Reason,
		State: documentTypeState,
		Apply: func(ctx context.Context, tx pgx.Tx) (*DocumentType, *DocumentType, error) {
			before, err := core.CollectReferenceRow[DocumentType](tx.Query(ctx, getDocumentTypeForUpdateSQL, pgx.NamedArgs{"code": code}))
			if err != nil {
				return nil, nil, fmt.Errorf("document type %q: %w", code, err)
			}
			after, err := core.CollectReferenceRow[DocumentType](tx.Query(ctx, updateDocumentTypeSQL, pgx.NamedArgs{"code": code, "label": in.Label, "description": in.Description, "category": in.Category, "is_active": in.IsActive}))
			return before, after, err
		},
	})
}
