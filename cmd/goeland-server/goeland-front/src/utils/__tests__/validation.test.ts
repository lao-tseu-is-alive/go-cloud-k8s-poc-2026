import { describe, expect, it } from 'vitest'
import { maxLength, minLength, required, sha256Rule } from '../validation'
import { t } from './i18nStub'

describe('validation rules', () => {
  it('required refuses blank values and non-text objects', () => {
    expect(required(t)('x')).toBe(true)
    expect(required(t)(0)).toBe(true)
    expect(required(t)('  ')).toBe('validation.required')
    expect(required(t)(undefined)).toBe('validation.required')
    expect(required(t)({})).toBe('validation.required')
  })

  it('bounds the length, an empty value passing minLength', () => {
    expect(maxLength(t, 3)('abc')).toBe(true)
    expect(maxLength(t, 3)('abcd')).toBe('validation.maxLength')
    expect(minLength(t, 3)('')).toBe(true)
    expect(minLength(t, 3)('ab')).toBe('validation.minLength')
  })

  it('checks a SHA-256 hex digest', () => {
    expect(sha256Rule(t)('a'.repeat(64))).toBe(true)
    expect(sha256Rule(t)('A'.repeat(64))).toBe(true)
    expect(sha256Rule(t)('g'.repeat(64))).toBe('validation.sha256')
    expect(sha256Rule(t)('')).toBe(true)
  })
})
