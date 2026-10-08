import { describe, expect, it } from 'vitest'
import { foldedFilter, foldText } from '../textSearch'

describe('foldText', () => {
  it('ignores case and accents', () => {
    expect(foldText('École Élémentaire')).toBe('ecole elementaire')
  })
})

describe('foldedFilter', () => {
  it('finds the query anywhere, accents and case aside', () => {
    expect(foldedFilter('Permis de construire', 'CONSTR')).toBe(10)
    expect(foldedFilter('Préavis', 'prea')).toBe(0)
  })

  it('answers -1 when nothing matches or the value is not text', () => {
    expect(foldedFilter('Préavis', 'zzz')).toBe(-1)
    expect(foldedFilter(undefined, 'a')).toBe(-1)
  })

  it('matches without highlight when folding changed the length', () => {
    expect(foldedFilter('Préavis', 'preavis')).toBe(true)
  })
})
