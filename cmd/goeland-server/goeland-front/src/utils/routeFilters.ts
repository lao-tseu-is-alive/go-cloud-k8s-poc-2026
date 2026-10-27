import type { LocationQuery } from 'vue-router'

/**
 * A list's filters in the URL (GLD-074): a search can be shared or found again
 * with the browser's history. Only the keys of the list's empty filters are
 * read and written, so other parameters (the tab of the results page) pass
 * through; the text filter is "q", as the global search writes it.
 */
export type ListFilters = Record<string, string | boolean | undefined>

/** The URL parameter of a filter key. */
function paramOf (key: string): string {
  return key === 'query' ? 'q' : key
}

/** The filters found in query, the others left at their empty value. */
export function filtersFromQuery<P extends ListFilters> (query: LocationQuery, empty: P): P {
  const filters: ListFilters = { ...empty }
  for (const key of Object.keys(empty)) {
    const value = query[paramOf(key)]
    if (typeof value !== 'string' || value === '') {
      continue
    }
    filters[key] = typeof empty[key] === 'boolean' ? value === 'true' : value
  }
  return filters as P
}

/** The URL parameters of the filters that differ from their empty value. */
export function filtersToQuery<P extends ListFilters> (filters: P, empty: P): Record<string, string> {
  const query: Record<string, string> = {}
  for (const key of Object.keys(empty)) {
    const value = filters[key]
    if ([undefined, '', empty[key]].includes(value)) {
      continue
    }
    query[paramOf(key)] = String(value)
  }
  return query
}

/** The parameters of query that are not filters of the list. */
export function otherParams (query: LocationQuery, empty: ListFilters): LocationQuery {
  const own = new Set(Object.keys(empty).map(key => paramOf(key)))
  return Object.fromEntries(Object.entries(query).filter(([key]) => !own.has(key)))
}

/** Whether two sets of filters ask for the same search. */
export function sameFilters<P extends ListFilters> (a: P, b: P, empty: P): boolean {
  return JSON.stringify(filtersToQuery(a, empty)) === JSON.stringify(filtersToQuery(b, empty))
}

/** How many of keys hold a value: the active filters behind « Plus de filtres ». */
export function activeCount (filters: ListFilters, keys: readonly string[]): number {
  return keys.filter(key => {
    const value = filters[key]
    return value !== undefined && value !== '' && value !== false
  }).length
}

/** The filters of components/core/RecordFilterFields, shared by every list. */
export const RECORD_FILTER_KEYS = ['createdFrom', 'createdTo', 'createdBy'] as const

/** The empty value of the shared record filters. */
export function emptyRecordFilters (): { createdFrom?: string, createdTo?: string, createdBy?: string } {
  return { createdFrom: undefined, createdTo: undefined, createdBy: undefined }
}
