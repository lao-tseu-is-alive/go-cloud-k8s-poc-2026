import type { SubjectKind, SubjectRef, SubjectSearchGroup } from '@/api/types'
/** The global search (GLD-064): the grouped suggestions of the app bar field. */
import type { useI18n } from 'vue-i18n'
import { formatTotal } from '@/utils/formatters'
import { kindIcon, kindListRoute, subjectRoute } from '@/utils/subjects'

/** A query shorter than this is not sent: one letter matches too much to help. */
export const MIN_QUERY_LENGTH = 2

/** A selectable suggestion: a subject, a filtered list or every result. */
export interface SearchSuggestion {
  value: string
  title: string
  subtitle?: string
  icon: string
  to: string
  /** The suggestion opens a list rather than a subject (shown as a link). */
  more?: boolean
}

/** A suggestion, a group title or a separator (VAutocomplete item types). */
export type SearchItem = SearchSuggestion | { type: 'subheader', title: string } | { type: 'divider' }

type Translate = ReturnType<typeof useI18n>['t']

/** The suggestion an item is, or an empty one for a title or a separator. */
export function asSuggestion (item: SearchItem): Partial<SearchSuggestion> {
  return 'value' in item ? item : {}
}

/** Whether query is worth sending. */
export function searchable (query: string): boolean {
  return query.trim().length >= MIN_QUERY_LENGTH
}

/** The route of the results page for query. */
export function resultsRoute (query: string): string {
  return `/search?q=${encodeURIComponent(query.trim())}`
}

/** The route of kind's list page filtered by query, or undefined when the kind has no list. */
export function filteredListRoute (kind: SubjectKind, query: string): string | undefined {
  const list = kindListRoute(kind)
  return list && `${list}?q=${encodeURIComponent(query.trim())}`
}

/** A subject's business reference, with its namespace. */
export function subjectReference (s: SubjectRef): string {
  return [s.businessRefNamespace, s.businessRef].filter(Boolean).join(' ')
}

/** A group's title: the kind in the plural and its count ("Affaires (500+)"). */
export function groupTitle (t: Translate, group: SubjectSearchGroup): string {
  const kind = t(`globalSearch.kinds.${group.kind}`)
  return `${kind} (${formatTotal(group.totalSize, group.totalSizeCapped)})`
}

function subjectSuggestion (s: SubjectRef): SearchSuggestion | undefined {
  const to = subjectRoute(s)
  if (!to) {
    return undefined
  }
  return { value: `subject:${s.id}`, title: s.displayLabel, subtitle: subjectReference(s) || undefined, icon: kindIcon(s.kind), to }
}

/**
 * The suggestions of the app bar field: every result first (Enter opens it),
 * then per kind with matches a title with the count, the best subjects and,
 * when there are more, a link to the kind's list filtered by the query.
 */
export function buildSearchItems (t: Translate, query: string, groups: SubjectSearchGroup[]): SearchItem[] {
  const found = groups.filter(g => (g.totalSize ?? 0) > 0)
  if (found.length === 0) {
    return []
  }
  const items: SearchItem[] = [{ value: 'all', title: t('globalSearch.all', { query: query.trim() }), icon: 'mdi-magnify', to: resultsRoute(query), more: true }]
  for (const group of found) {
    items.push({ type: 'divider' }, { type: 'subheader', title: groupTitle(t, group) })
    for (const s of group.subjects ?? []) {
      const suggestion = subjectSuggestion(s)
      if (suggestion) {
        items.push(suggestion)
      }
    }
    const list = filteredListRoute(group.kind, query)
    if (list && (group.totalSize ?? 0) > (group.subjects?.length ?? 0)) {
      items.push({
        value: `list:${group.kind}`,
        title: t('globalSearch.seeAll', { count: formatTotal(group.totalSize, group.totalSizeCapped), list: t(`globalSearch.kinds.${group.kind}`) }),
        icon: 'mdi-arrow-right',
        to: list,
        more: true,
      })
    }
  }
  return items
}
