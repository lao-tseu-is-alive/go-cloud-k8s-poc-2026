package core

import (
	"fmt"
	"slices"
	"strings"
)

// Sort is the order of a list (a sortable column of the UI): one of the
// fields the list allows, ascending or descending. NULL values come last
// either way, and the id breaks ties, so paging is stable.
type Sort struct {
	// Field is the API name of the sorted field (e.g. "title").
	Field string
	// Desc sorts in descending order.
	Desc bool
}

// ParseOrderBy reads an order_by request field, "<field>" or "<field> asc|desc"
// (AIP-132), against the fields a list allows; an empty value gives def. An
// unknown field or direction is ErrInvalidInput.
func ParseOrderBy(orderBy string, allowed []string, def Sort) (Sort, error) {
	parts := strings.Fields(strings.ToLower(orderBy))
	if len(parts) == 0 {
		return def, nil
	}
	if len(parts) > 2 || !slices.Contains(allowed, parts[0]) {
		return Sort{}, fmt.Errorf("%w: order_by must be one of %s, optionally followed by asc or desc", ErrInvalidInput, strings.Join(allowed, ", "))
	}
	s := Sort{Field: parts[0]}
	if len(parts) == 2 {
		switch parts[1] {
		case "asc":
		case "desc":
			s.Desc = true
		default:
			return Sort{}, fmt.Errorf("%w: the order_by direction must be asc or desc", ErrInvalidInput)
		}
	}
	return s, nil
}

// SortField is how a list sorts on one field: the SQL expression of the sort
// key, and a join the expression needs (empty when none).
type SortField struct {
	// Expr is the SQL expression, aliased sort_key by the query builders.
	Expr string
	// Join is the join clause the expression needs.
	Join string
	// Nullable marks a key that may be NULL: descending, NULL values are put
	// last explicitly. A NOT NULL key keeps the plain direction, so an
	// ascending index is also read backwards (asking NULLS LAST on it made a
	// descending title sort 15 times slower).
	Nullable bool
}

// SortOrder is the direction of a sort and whether NULL values must be put last.
type SortOrder struct {
	// Desc sorts in descending order.
	Desc bool
	// NullsLast puts NULL values last when descending (they are last ascending anyway).
	NullsLast bool
}

// Order is the order of f in direction desc.
func (f SortField) Order(desc bool) SortOrder {
	return SortOrder{Desc: desc, NullsLast: f.Nullable}
}

// SortedQueries builds once the query of every sort of a list (each field in
// both directions), so a request only picks one: build gets the field and
// the direction.
func SortedQueries(fields map[string]SortField, build func(SortField, bool) string) map[Sort]string {
	out := make(map[Sort]string, 2*len(fields))
	for name, f := range fields {
		for _, desc := range []bool{false, true} {
			out[Sort{Field: name, Desc: desc}] = build(f, desc)
		}
	}
	return out
}

// SortNames lists the field names of a sort map, sorted, for ParseOrderBy.
func SortNames(fields map[string]SortField) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// OrderDirection is the SQL of a sort direction, NULL values last.
func OrderDirection(o SortOrder) string {
	switch {
	case o.Desc && o.NullsLast:
		return " DESC NULLS LAST"
	case o.Desc:
		return " DESC"
	default:
		return " ASC"
	}
}

// SortedQuery returns the query of sort s, or of def when s is not one of them
// (a filter built without the service).
func SortedQuery(queries map[Sort]string, s, def Sort) string {
	if q, ok := queries[s]; ok {
		return q
	}
	return queries[def]
}
