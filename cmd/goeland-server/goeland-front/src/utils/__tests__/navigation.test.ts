import { describe, expect, it } from 'vitest'
import { NAV_ITEMS, navItemsFor } from '../navigation'

describe('navigation', () => {
  it('shows the administration to administrators only', () => {
    expect(navItemsFor(true, true)).toHaveLength(NAV_ITEMS.length)
    expect(navItemsFor(false).some(i => i.to === '/admin')).toBe(false)
    expect(navItemsFor(false)).toHaveLength(NAV_ITEMS.length - 2)
  })

  it('shows the retention deadlines to records managers', () => {
    expect(navItemsFor(false, true).some(i => i.to === '/retention')).toBe(true)
    expect(navItemsFor(false, true).some(i => i.to === '/admin')).toBe(false)
    expect(navItemsFor(false, false).some(i => i.to === '/retention')).toBe(false)
  })

  it('opens on the dashboard, which holds the tasks', () => {
    expect(NAV_ITEMS[0]?.to).toBe('/home')
    expect(NAV_ITEMS.some(i => i.to === '/tasks/mine')).toBe(false)
  })
})
