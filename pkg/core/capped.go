package core

// MaxCountedTotal bounds the totals of searches and lists (GLD-053): at
// production volume, counting every match of an unfiltered case search
// (~490k readable cases) took seconds. Beyond it, a total is a lower bound
// (TotalCapped) and paging stops there: the search must be refined.
const MaxCountedTotal = 10000

// CountLimit is the @count_limit argument of a CappedPageSQL query.
const CountLimit = MaxCountedTotal + 1

// CappedPageSQL builds a paged query whose total counts at most CountLimit
// matches. match selects "<alias>.id AS id, <sort expression> AS sort_key"
// from the filtered rows; it is ordered by sort_key (descending when desc)
// then id, and cut at @count_limit, so an index on the sort key ends the scan
// early. The page (@limit, @offset) is read from those matches and projected
// with columns over table aliased alias; every row carries total_count.
func CappedPageSQL(match, columns, table, alias string, desc bool) string {
	direction := ""
	if desc {
		direction = " DESC"
	}
	order := "sort_key" + direction + ", id" + direction
	return `
WITH matched AS (
` + match + `
ORDER BY ` + order + `
LIMIT @count_limit)
SELECT ` + columns + `, (SELECT count(*) FROM matched) AS total_count
FROM (SELECT id, sort_key FROM matched ORDER BY ` + order + ` LIMIT @limit OFFSET @offset) m
JOIN ` + table + ` ` + alias + ` ON ` + alias + `.id = m.id
ORDER BY m.sort_key` + direction + `, m.id` + direction + `;`
}

// MetadataLateralSQL joins the governance of subject idExpr as rm (deleted_at,
// confidentiality_level) for a CappedPageSQL match. A LATERAL subquery with
// LIMIT cannot be flattened, so the scan follows the index on the sort key and
// stops at the count limit, where a plain join let the planner hash-join every
// subject first (GLD-053: 0.8 s → 0.05 s on the unfiltered case search).
func MetadataLateralSQL(idExpr string) string {
	return `CROSS JOIN LATERAL (SELECT r.deleted_at, r.confidentiality_level FROM record_metadata r WHERE r.subject_id = ` + idExpr + ` LIMIT 1) rm`
}

// CapTotal turns the total of a CappedPageSQL query into the reported total
// and whether it is a lower bound.
func CapTotal(total int32) (int32, bool) {
	if total > MaxCountedTotal {
		return MaxCountedTotal, true
	}
	return total, false
}
