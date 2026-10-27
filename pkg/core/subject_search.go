package core

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Global search (GLD-064): one query field over every kind of subject.
// subject_ref holds the label and business reference of every subject, kept
// current by core, so it is the search table; migration 0032 indexes it per
// kind. Kinds added later are searchable without code. Descriptions and
// kind-specific fields stay with the search of each domain.

// DefaultSearchKinds are the kinds searched when a request names none, in the
// order of the result groups.
var DefaultSearchKinds = []SubjectKind{SubjectKindCase, SubjectKindActor, SubjectKindDocument, SubjectKindThing}

const (
	// defaultSearchPerKind is the number of subjects per group by default.
	defaultSearchPerKind = 5
	// maxSearchPerKind bounds the subjects per group.
	maxSearchPerKind = 25
	// maxSearchTotal bounds the count of a group: a type-ahead query matching
	// more must be refined, and counting further on 2.3M documents costs.
	maxSearchTotal = 500
)

// SubjectSearchFilter selects a global search.
type SubjectSearchFilter struct {
	// Viewer is the caller; only the subjects it may read are found.
	Viewer Viewer
	// Query is the user's text (QueryWords splits it).
	Query string
	// Kinds are the kinds searched, one group each; empty means DefaultSearchKinds.
	Kinds []SubjectKind
	// LimitPerKind is the number of subjects per group; 0 means the default.
	LimitPerKind int
}

// SubjectSearchGroup is the result of a global search for one kind.
type SubjectSearchGroup struct {
	// Kind is the kind of the subjects.
	Kind SubjectKind
	// Subjects are the best matches, best first.
	Subjects []*SubjectRef
	// Total counts the readable matches, at most maxSearchTotal.
	Total int32
	// TotalCapped reports more than maxSearchTotal matches.
	TotalCapped bool
}

// SubjectSearchHit is one subject found by a global search.
type SubjectSearchHit struct {
	SubjectRef
	// Ord is the 1-based position of the subject's kind in the filter.
	Ord int64 `db:"ord"`
	// TotalCount counts the readable matches of that kind, at most maxSearchTotal+1.
	TotalCount int64 `db:"total_count"`
}

// SearchSubjects searches the subjects of several kinds by label and business
// reference and returns one group per kind, in the requested order. A query
// without a word finds nothing.
func (s *Service) SearchSubjects(ctx context.Context, filter SubjectSearchFilter) ([]*SubjectSearchGroup, error) {
	if len(filter.Kinds) == 0 {
		filter.Kinds = DefaultSearchKinds
	}
	for _, k := range filter.Kinds {
		if !k.Valid() {
			return nil, fmt.Errorf("%w: unknown subject kind %q", ErrInvalidInput, k)
		}
	}
	filter.Kinds = slices.Compact(slices.Clone(filter.Kinds))
	switch {
	case filter.LimitPerKind <= 0:
		filter.LimitPerKind = defaultSearchPerKind
	case filter.LimitPerKind > maxSearchPerKind:
		filter.LimitPerKind = maxSearchPerKind
	}
	groups := make([]*SubjectSearchGroup, len(filter.Kinds))
	for i, k := range filter.Kinds {
		groups[i] = &SubjectSearchGroup{Kind: k, Subjects: []*SubjectRef{}}
	}
	if len(QueryWords(filter.Query)) == 0 {
		return groups, nil
	}
	rows, err := s.repo.SearchSubjects(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("search subjects: %w", err)
	}
	for _, row := range rows {
		g := groups[row.Ord-1]
		ref := row.SubjectRef
		g.Subjects = append(g.Subjects, &ref)
		g.Total = int32(min(row.TotalCount, maxSearchTotal))
		g.TotalCapped = row.TotalCount > maxSearchTotal
	}
	return groups, nil
}

// SearchSubjects runs a global search (searchSubjectsSQL); filter is complete.
func (r *PostgresRepository) SearchSubjects(ctx context.Context, filter SubjectSearchFilter) ([]SubjectSearchHit, error) {
	kinds := make([]string, len(filter.Kinds))
	for i, k := range filter.Kinds {
		kinds[i] = string(k)
	}
	query := strings.TrimSpace(filter.Query)
	rows, err := r.pool.Query(ctx, searchSubjectsSQL, filter.Viewer.AddTo(pgx.NamedArgs{
		"kinds":        kinds,
		"text_query":   PrefixTSQuery(query),
		"business_ref": query,
		"label_prefix": strings.ToLower(QueryWords(query)[0]) + "%",
		"limit":        filter.LimitPerKind,
		"count_limit":  maxSearchTotal + 1,
	}))
	if err != nil {
		return nil, fmt.Errorf("search subjects: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToStructByNameLax[SubjectSearchHit])
}

// subjectSearchVectorSQL is the indexed search vector of subject_ref alias a;
// it must stay identical to the expression of idx_subject_ref_search
// (migration 0032), or the search scans the table.
func subjectSearchVectorSQL(a string) string {
	return `(to_tsvector('simple', regexp_replace(immutable_unaccent(` + a + `.display_label || ' ' || ` + a + `.business_ref), '[^[:alnum:]]+', ' ', 'g'))
       || ('''§' || lower(` + a + `.kind) || '''')::tsvector)`
}
