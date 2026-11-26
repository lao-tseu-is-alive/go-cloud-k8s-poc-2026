import { describe, expect, it } from 'vitest'
import { activityById, lastActivity } from '../activity'

describe('activity', () => {
  it('keeps the latest of the last change and the last consultation', () => {
    expect(lastActivity()).toBeNull()
    expect(lastActivity({ subjectId: 'a' })).toBeNull()
    expect(lastActivity({ lastViewedAt: '2026-11-26T10:00:00Z' })).toEqual({ kind: 'viewed', at: '2026-11-26T10:00:00Z' })
    expect(lastActivity({ lastChangedAt: '2026-11-25T10:00:00Z', lastViewedAt: '2026-11-26T10:00:00Z' })?.kind).toBe('viewed')
    expect(lastActivity({ lastChangedAt: '2026-11-26T11:00:00Z', lastViewedAt: '2026-11-26T10:00:00Z' })?.kind).toBe('changed')
    expect(lastActivity({ lastChangedAt: '2026-11-26T10:00:00Z', lastViewedAt: '2026-11-26T10:00:00Z' })?.kind).toBe('changed')
  })

  it('indexes the activities by subject', () => {
    expect(activityById([{ subjectId: 'a', lastViewedAt: 'x' }, {}])).toEqual({ a: { subjectId: 'a', lastViewedAt: 'x' } })
    expect(activityById()).toEqual({})
  })
})
