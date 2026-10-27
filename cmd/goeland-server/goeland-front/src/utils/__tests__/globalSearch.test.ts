import type { SubjectSearchGroup } from '@/api/types'
import { describe, expect, it } from 'vitest'
import { buildSearchItems, filteredListRoute, resultsRoute, searchable, subjectReference } from '../globalSearch'
import { t } from './i18nStub'

const cases: SubjectSearchGroup = {
  kind: 'SUBJECT_KIND_CASE',
  totalSize: 500,
  totalSizeCapped: true,
  subjects: [{ id: 'c1', kind: 'SUBJECT_KIND_CASE', displayLabel: 'Église de Rhodanie', businessRef: '2026-000123', businessRefNamespace: 'OPC' }],
}
const actors: SubjectSearchGroup = {
  kind: 'SUBJECT_KIND_ACTOR',
  totalSize: 1,
  subjects: [{ id: 'a1', kind: 'SUBJECT_KIND_ACTOR', displayLabel: 'Bureau Rhodanie SA' }],
}
const noDocuments: SubjectSearchGroup = { kind: 'SUBJECT_KIND_DOCUMENT', totalSize: 0, subjects: [] }

describe('global search suggestions (GLD-064)', () => {
  it('sends only a query of at least two characters', () => {
    expect(searchable(' r ')).toBe(false)
    expect(searchable('rh')).toBe(true)
  })

  it('builds the routes of the results page and of a filtered list', () => {
    expect(resultsRoute(' rue & co ')).toBe('/search?q=rue%20%26%20co')
    expect(filteredListRoute('SUBJECT_KIND_CASE', 'rhod')).toBe('/cases?q=rhod')
    expect(filteredListRoute('SUBJECT_KIND_GROUP', 'rhod')).toBeUndefined()
  })

  it('shows the reference with its namespace', () => {
    expect(subjectReference(cases.subjects![0]!)).toBe('OPC 2026-000123')
    expect(subjectReference(actors.subjects![0]!)).toBe('')
  })

  it('offers every result first, then the groups with matches only', () => {
    const items = buildSearchItems(t, ' rhod ', [cases, noDocuments, actors])
    expect(items.map(i => ('value' in i ? i.value : i.type))).toEqual([
      'all',
      'divider', 'subheader', 'subject:c1', 'list:SUBJECT_KIND_CASE',
      'divider', 'subheader', 'subject:a1',
    ])
    expect(items[0]).toMatchObject({ to: '/search?q=rhod', more: true })
    expect(items[2]).toMatchObject({ title: 'globalSearch.kinds.SUBJECT_KIND_CASE (500+)' })
    expect(items[3]).toMatchObject({ to: '/cases/c1', subtitle: 'OPC 2026-000123' })
    expect(items[4]).toMatchObject({ to: '/cases?q=rhod', more: true })
  })

  it('offers nothing when no group has a match', () => {
    expect(buildSearchItems(t, 'zzz', [noDocuments])).toEqual([])
  })
})
