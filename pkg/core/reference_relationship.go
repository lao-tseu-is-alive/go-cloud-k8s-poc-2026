package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// RelationshipTypeInput is a new relationship type (administrators only).
type RelationshipTypeInput struct {
	// Code is the immutable key (see ValidateReferenceCode).
	Code string
	// Label is the required human label.
	Label string
	// SourceKind is the required kind of source subjects.
	SourceKind SubjectKind
	// TargetKind is the required kind of target subjects.
	TargetKind SubjectKind
	// IsDirected reports whether the edge reads one way only.
	IsDirected bool
	// InverseLabel is the label read from target to source.
	InverseLabel string
	// Description documents the business meaning.
	Description string
	// OperatorID is the administrator, set server-side.
	OperatorID string
	// Reason is the justification recorded in the reference change log.
	Reason string
}

// RelationshipTypeUpdate changes a relationship type; nil fields are kept.
// Code, kinds and direction are immutable: existing edges rely on them.
type RelationshipTypeUpdate struct {
	// Label replaces the label when non-nil (not blank).
	Label *string
	// InverseLabel replaces the inverse label when non-nil.
	InverseLabel *string
	// Description replaces the description when non-nil.
	Description *string
	// IsActive activates or deactivates the type when non-nil.
	IsActive *bool
	// OperatorID is the administrator, set server-side.
	OperatorID string
	// Reason is the justification recorded in the reference change log.
	Reason string
}

// relationshipTypeState is the logged state of a relationship type.
func relationshipTypeState(rt *RelationshipType) map[string]any {
	return map[string]any{
		"label": rt.Label, "source_kind": string(rt.SourceKind), "target_kind": string(rt.TargetKind),
		"is_directed": rt.IsDirected, "inverse_label": rt.InverseLabel, "description": rt.Description, "is_active": rt.IsActive,
	}
}

// CreateRelationshipType validates and adds a relationship type.
func (s *Service) CreateRelationshipType(ctx context.Context, in RelationshipTypeInput) (*RelationshipType, *ReferenceChange, error) {
	var err error
	in.Code = strings.TrimSpace(in.Code)
	if err = ValidateReferenceCode(in.Code); err != nil {
		return nil, nil, err
	}
	if !in.SourceKind.Valid() || !in.TargetKind.Valid() {
		return nil, nil, fmt.Errorf("%w: source_kind and target_kind are required", ErrInvalidInput)
	}
	if in.Label, err = NormalizeReferenceText("label", in.Label, MaxReferenceLabelLength, true); err != nil {
		return nil, nil, err
	}
	if in.InverseLabel, err = NormalizeReferenceText("inverse_label", in.InverseLabel, MaxReferenceLabelLength, false); err != nil {
		return nil, nil, err
	}
	if in.Description, err = NormalizeReferenceText("description", in.Description, MaxReferenceDescriptionLength, false); err != nil {
		return nil, nil, err
	}
	return s.repo.CreateRelationshipType(ctx, in)
}

// UpdateRelationshipType validates and applies a relationship type change.
func (s *Service) UpdateRelationshipType(ctx context.Context, code string, in RelationshipTypeUpdate) (*RelationshipType, *ReferenceChange, error) {
	if err := NormalizeOptionalReferenceText("label", &in.Label, MaxReferenceLabelLength, true); err != nil {
		return nil, nil, err
	}
	if err := NormalizeOptionalReferenceText("inverse_label", &in.InverseLabel, MaxReferenceLabelLength, false); err != nil {
		return nil, nil, err
	}
	if err := NormalizeOptionalReferenceText("description", &in.Description, MaxReferenceDescriptionLength, false); err != nil {
		return nil, nil, err
	}
	return s.repo.UpdateRelationshipType(ctx, strings.TrimSpace(code), in)
}

// ListReferenceChanges pages the reference change log, newest first.
func (s *Service) ListReferenceChanges(ctx context.Context, filter ReferenceFilter) (ReferenceResult, error) {
	limit, err := NormalizePageSize(filter.Limit)
	if err != nil {
		return ReferenceResult{}, err
	}
	filter.Limit, filter.Offset = limit, max(filter.Offset, 0)
	return s.repo.ListReferenceChanges(ctx, filter)
}

// CreateRelationshipType inserts the type and its REFERENCE_CREATED log entry.
func (r *PostgresRepository) CreateRelationshipType(ctx context.Context, in RelationshipTypeInput) (*RelationshipType, *ReferenceChange, error) {
	return MutateReference(ctx, r.pool, ReferenceMutation[RelationshipType]{
		Catalogue: CatalogueRelationshipType, Code: in.Code, OperatorID: in.OperatorID, Reason: in.Reason,
		State: relationshipTypeState,
		Apply: func(ctx context.Context, tx pgx.Tx) (*RelationshipType, *RelationshipType, error) {
			rows, err := tx.Query(ctx, insertRelationshipTypeSQL, pgx.NamedArgs{
				"code": in.Code, "label": in.Label, "source_kind": string(in.SourceKind), "target_kind": string(in.TargetKind),
				"is_directed": in.IsDirected, "inverse_label": in.InverseLabel, "description": in.Description,
			})
			if err != nil {
				return nil, nil, MapReferenceConflict(err, CatalogueRelationshipType, in.Code)
			}
			rt, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[RelationshipType])
			return nil, rt, MapReferenceConflict(err, CatalogueRelationshipType, in.Code)
		},
	})
}

// UpdateRelationshipType locks the type, applies the change and logs it.
func (r *PostgresRepository) UpdateRelationshipType(ctx context.Context, code string, in RelationshipTypeUpdate) (*RelationshipType, *ReferenceChange, error) {
	return MutateReference(ctx, r.pool, ReferenceMutation[RelationshipType]{
		Catalogue: CatalogueRelationshipType, Code: code, OperatorID: in.OperatorID, Reason: in.Reason,
		State: relationshipTypeState,
		Apply: func(ctx context.Context, tx pgx.Tx) (*RelationshipType, *RelationshipType, error) {
			before, err := CollectReferenceRow[RelationshipType](tx.Query(ctx, getRelationshipTypeForUpdateSQL, pgx.NamedArgs{"code": code}))
			if err != nil {
				return nil, nil, err
			}
			after, err := CollectReferenceRow[RelationshipType](tx.Query(ctx, updateRelationshipTypeSQL, pgx.NamedArgs{
				"code": code, "label": in.Label, "inverse_label": in.InverseLabel, "description": in.Description, "is_active": in.IsActive,
			}))
			return before, after, err
		},
	})
}

// ListReferenceChanges returns a page of the reference change log.
func (r *PostgresRepository) ListReferenceChanges(ctx context.Context, filter ReferenceFilter) (ReferenceResult, error) {
	rows, err := r.pool.Query(ctx, listReferenceChangesSQL, pgx.NamedArgs{
		"catalogue": filter.Catalogue, "limit": filter.Limit, "offset": filter.Offset,
	})
	if err != nil {
		return ReferenceResult{}, fmt.Errorf("list reference changes: %w", err)
	}
	type row struct {
		ReferenceChange
		TotalSize int32 `db:"total_size"`
	}
	listed, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[row])
	if err != nil {
		return ReferenceResult{}, fmt.Errorf("read reference changes: %w", err)
	}
	result := ReferenceResult{Changes: make([]*ReferenceChange, len(listed))}
	for i := range listed {
		change := listed[i].ReferenceChange
		result.Changes[i] = &change
		result.TotalSize = listed[i].TotalSize
	}
	return result, nil
}
