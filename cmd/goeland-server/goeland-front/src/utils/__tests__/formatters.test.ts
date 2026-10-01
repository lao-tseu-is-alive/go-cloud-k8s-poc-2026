import { describe, expect, it } from 'vitest'
import { formatBytes, formatDate, formatDateTime, formatTotal, shortHash } from '../formatters'

describe('formatters', () => {
  it('formats int64-as-string byte counts', () => {
    expect(formatBytes('38')).toBe('38 B')
    expect(formatBytes(1536)).toBe('1.5 KiB')
    expect(formatBytes(`${5 * 1024 * 1024}`)).toBe('5.0 MiB')
    expect(formatBytes(undefined)).toBe('—')
    expect(formatBytes('abc')).toBe('abc')
  })

  it('shortens long digests only', () => {
    expect(shortHash('0123456789abcdef0123456789abcdef')).toBe('01234567…89abcdef')
    expect(shortHash('short')).toBe('short')
    expect(shortHash(undefined)).toBe('—')
  })

  it('keeps unparseable dates and dashes absent ones', () => {
    expect(formatDate(undefined)).toBe('—')
    expect(formatDateTime('')).toBe('—')
    expect(formatDate('not a date')).toBe('not a date')
    expect(formatDate('2026-09-30T08:00:00Z')).not.toBe('2026-09-30T08:00:00Z')
  })
})

describe('formatTotal', () => {
  it('prints an exact total', () => {
    expect(formatTotal(42, false)).toBe('42')
    expect(formatTotal(undefined)).toBe('0')
  })

  it('marks a capped total as a lower bound', () => {
    expect(formatTotal(10_000, true)).toMatch(/^10.000\+$/)
  })
})
