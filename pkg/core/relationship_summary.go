package core

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Relationship groups (GLD-063): a detail page shows a subject's edges grouped
// by relationship type and direction, each group titled from the subject's
// point of view and counted before it is opened. Counts cover only the edges
// whose other end the viewer may read, exactly like the lists that open them.

// RelationshipGroup counts the edges of one type in one direction.
type RelationshipGroup struct {
	// RelationshipType is the type of the edges.
	RelationshipType *RelationshipType
	// Outgoing is true when the subject is the source of the edges.
	Outgoing bool
	// OtherKind is the kind of the other ends.
	OtherKind SubjectKind
	// Count is the number of edges.
	Count int32
	// OpenCount is the number of other ends that are cases not closed.
	OpenCount int32
	// Families splits the edges by case type or document type, by label.
	Families []RelationshipFamily
}

// RelationshipFamily is the share of a group of one case or document type.
type RelationshipFamily struct {
	// Code is the type code.
	Code string
	// Label is the type label.
	Label string
	// Count is the number of edges.
	Count int32
	// OpenCount is the number of those that are cases not closed.
	OpenCount int32
}

// RelatedDetail is what a relationship list shows of the other end beyond its
// reference and label.
type RelatedDetail struct {
	// SubjectID is the other end.
	SubjectID uuid.UUID `db:"id"`
	// TypeCode is the case, document or thing type; empty otherwise.
	TypeCode string `db:"type_code"`
	// TypeLabel is the label of that type.
	TypeLabel string `db:"type_label"`
	// CaseStatus is OPEN, IN_PROGRESS, SUSPENDED or CLOSED for a case; empty otherwise.
	CaseStatus string `db:"case_status"`
	// Date is a case's opening date or a document's official date (YYYY-MM-DD).
	Date string `db:"date"`
}

// summaryRow is one row of summarizeRelationshipsSQL: a group and family.
type summaryRow struct {
	// TypeID is the relationship type.
	TypeID uuid.UUID `db:"type_id"`
	// Outgoing is true when the subject is the source.
	Outgoing bool `db:"outgoing"`
	// OtherKind is the kind of the other ends.
	OtherKind SubjectKind `db:"other_kind"`
	// FamCode is the case or document type code; empty for other kinds.
	FamCode string `db:"fam_code"`
	// FamLabel is that type's label.
	FamLabel string `db:"fam_label"`
	// Count is the number of edges of the row.
	Count int32 `db:"n"`
	// OpenCount is the number of those whose other end is a case not closed.
	OpenCount int32 `db:"open_n"`
}

// SummarizeRelationships returns the relationship groups of a subject as
// viewer sees them, by type label then direction.
func (s *Service) SummarizeRelationships(ctx context.Context, subjectID uuid.UUID, viewer Viewer) ([]*RelationshipGroup, error) {
	if subjectID == uuid.Nil {
		return nil, fmt.Errorf("%w: subject id is required", ErrInvalidInput)
	}
	return s.repo.SummarizeRelationships(ctx, subjectID, viewer)
}

// SummarizeRelationships counts the edges of a subject per type, direction
// and case or document type, keeping those whose other end viewer may read.
func (r *PostgresRepository) SummarizeRelationships(ctx context.Context, subjectID uuid.UUID, viewer Viewer) ([]*RelationshipGroup, error) {
	rows, err := r.pool.Query(ctx, summarizeRelationshipsSQL, viewer.AddTo(pgx.NamedArgs{"subject_id": subjectID}))
	if err != nil {
		return nil, fmt.Errorf("summarize relationships: %w", err)
	}
	listed, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[summaryRow])
	if err != nil {
		return nil, fmt.Errorf("read relationship summary: %w", err)
	}
	groups, typeIDs := groupSummaryRows(listed)
	typeRows, err := r.pool.Query(ctx, getRelationshipTypesByIDsSQL, pgx.NamedArgs{"ids": typeIDs})
	if err != nil {
		return nil, fmt.Errorf("summary relationship types: %w", err)
	}
	types, err := pgx.CollectRows(typeRows, pgx.RowToAddrOfStructByNameLax[RelationshipType])
	if err != nil {
		return nil, fmt.Errorf("read summary relationship types: %w", err)
	}
	byID := make(map[uuid.UUID]*RelationshipType, len(types))
	for _, t := range types {
		byID[t.ID] = t
	}
	for _, g := range groups {
		g.RelationshipType = byID[g.RelationshipType.ID]
	}
	slices.SortStableFunc(groups, func(a, b *RelationshipGroup) int {
		return cmp.Or(cmp.Compare(a.RelationshipType.Label, b.RelationshipType.Label), cmp.Compare(boolRank(b.Outgoing), boolRank(a.Outgoing)))
	})
	return groups, nil
}

// groupSummaryRows folds the per-family rows into groups (type and direction)
// and returns the type ids to hydrate.
func groupSummaryRows(rows []summaryRow) ([]*RelationshipGroup, []uuid.UUID) {
	type key struct {
		typeID   uuid.UUID
		outgoing bool
	}
	index := map[key]*RelationshipGroup{}
	var groups []*RelationshipGroup
	var typeIDs []uuid.UUID
	for _, row := range rows {
		k := key{row.TypeID, row.Outgoing}
		g, ok := index[k]
		if !ok {
			g = &RelationshipGroup{RelationshipType: &RelationshipType{ID: row.TypeID}, Outgoing: row.Outgoing, OtherKind: row.OtherKind}
			index[k] = g
			groups = append(groups, g)
			typeIDs = append(typeIDs, row.TypeID)
		}
		g.Count += row.Count
		g.OpenCount += row.OpenCount
		if row.FamCode != "" {
			g.Families = append(g.Families, RelationshipFamily{Code: row.FamCode, Label: row.FamLabel, Count: row.Count, OpenCount: row.OpenCount})
		}
	}
	return groups, typeIDs
}

// boolRank orders true before false.
func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}

// relatedDetails returns the detail of each subject of ids, by id.
func (r *PostgresRepository) relatedDetails(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*RelatedDetail, error) {
	return CollectIndexedTx(ctx, r.pool, relatedDetailsSQL, ids, func(d *RelatedDetail) uuid.UUID { return d.SubjectID })
}

// attachRelatedDetails sets OtherDetail on each edge, the other end seen from subjectID.
func (r *PostgresRepository) attachRelatedDetails(ctx context.Context, rels []*SubjectRelationship, subjectID uuid.UUID) error {
	others := make([]uuid.UUID, 0, len(rels))
	for _, rel := range rels {
		others = append(others, rel.otherEnd(subjectID))
	}
	details, err := r.relatedDetails(ctx, others)
	if err != nil {
		return fmt.Errorf("related details: %w", err)
	}
	for _, rel := range rels {
		rel.OtherDetail = details[rel.otherEnd(subjectID)]
	}
	return nil
}

// otherEnd is the end of the edge that is not subjectID.
func (rel *SubjectRelationship) otherEnd(subjectID uuid.UUID) uuid.UUID {
	if rel.SourceSubjectID == subjectID {
		return rel.TargetSubjectID
	}
	return rel.SourceSubjectID
}
