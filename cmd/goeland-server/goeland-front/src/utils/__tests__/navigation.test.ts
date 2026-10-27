import { describe, expect, it } from 'vitest'
import { NAV_ITEMS, navItemsFor } from '../navigation'

describe('navigation', () => {
  it('shows the administration to administrators only', () => {
    expect(navItemsFor(true)).toHaveLength(NAV_ITEMS.length)
    expect(navItemsFor(false).some(i => i.to === '/admin')).toBe(false)
    expect(navItemsFor(false)).toHaveLength(NAV_ITEMS.length - 1)
  })
})
