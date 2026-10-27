import { describe, expect, it } from 'vitest'
import { activeCount, filtersFromQuery, filtersToQuery, otherParams, sameFilters } from '../routeFilters'

const empty = () => ({ query: '', status: undefined as string | undefined, createdFrom: undefined as string | undefined, includeDeleted: false })

describe('routeFilters', () => {
  it('reads the declared filters, q for the text, booleans from "true"', () => {
    expect(filtersFromQuery({ q: 'ge 17', status: 'CASE_STATUS_OPEN', includeDeleted: 'true', tab: 'CASE' }, empty()))
      .toEqual({ query: 'ge 17', status: 'CASE_STATUS_OPEN', createdFrom: undefined, includeDeleted: true })
  })

  it('ignores empty and repeated parameters', () => {
    expect(filtersFromQuery({ q: '', status: ['A', 'B'] }, empty())).toEqual(empty())
  })

  it('writes only the filters that differ from their empty value', () => {
    expect(filtersToQuery({ ...empty(), query: 'ge', createdFrom: '2020-01-01' }, empty()))
      .toEqual({ q: 'ge', createdFrom: '2020-01-01' })
    expect(filtersToQuery(empty(), empty())).toEqual({})
  })

  it('round-trips through the URL', () => {
    const filters = { query: 'rhône', status: 'CASE_STATUS_CLOSED', createdFrom: '2019-03-01', includeDeleted: true }
    expect(filtersFromQuery(filtersToQuery(filters, empty()), empty())).toEqual(filters)
  })

  it('keeps the parameters of others', () => {
    expect(otherParams({ q: 'x', tab: 'DOCUMENT', status: 'A' }, empty())).toEqual({ tab: 'DOCUMENT' })
  })

  it('compares searches, not objects', () => {
    expect(sameFilters({ ...empty(), status: '' }, empty(), empty())).toBe(true)
    expect(sameFilters({ ...empty(), query: 'a' }, empty(), empty())).toBe(false)
  })

  it('counts the active filters among keys', () => {
    expect(activeCount({ createdFrom: '2020-01-01', createdTo: '', createdBy: 'u1', onlyFinal: false }, ['createdFrom', 'createdTo', 'createdBy', 'onlyFinal'])).toBe(2)
  })
})
