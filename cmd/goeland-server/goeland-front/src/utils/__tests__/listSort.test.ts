import { describe, expect, it } from 'vitest'
import { nextSort, sortRows, toOrderBy } from '../listSort'

describe('listSort', () => {
  it('turns a sort into order_by', () => {
    expect(toOrderBy(undefined)).toBeUndefined()
    expect(toOrderBy({ field: 'title', desc: false })).toBe('title')
    expect(toOrderBy({ field: 'title', desc: true })).toBe('title desc')
  })

  it('sorts ascending first, then flips', () => {
    const first = nextSort(undefined, 'title')
    expect(first).toEqual({ field: 'title', desc: false })
    expect(nextSort(first, 'title')).toEqual({ field: 'title', desc: true })
    expect(nextSort(first, 'status')).toEqual({ field: 'status', desc: false })
  })

  it('sorts loaded rows like the server: accents, numbers, empty last', () => {
    const rows = [{ n: 'école' }, { n: '' }, { n: 'Abc' }, { n: 'zoo' }, { n: 'Ecole 10' }, { n: 'Ecole 9' }]
    const names = (desc: boolean) => sortRows(rows, { field: 'n', desc }, r => r.n).map(r => r.n)
    expect(names(false)).toEqual(['Abc', 'école', 'Ecole 9', 'Ecole 10', 'zoo', ''])
    expect(names(true).at(-1)).toBe('')
    expect(sortRows(rows, undefined, r => r.n)).toBe(rows)
  })
})
