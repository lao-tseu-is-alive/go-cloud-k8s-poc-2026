import { describe, expect, it } from 'vitest'
import { geometryRule, parseGeometry, SAMPLE_POLYGON, svgGeometry, swissMapPointUrl } from '../geometry'
import { t } from './i18nStub'

describe('geometry', () => {
  it('parses points and polygon rings', () => {
    expect(parseGeometry('{"type":"Point","coordinates":[2538000,1152000]}')).toEqual({ type: 'Point', points: [[2_538_000, 1_152_000]], paths: [], filled: false })
    const polygon = parseGeometry(SAMPLE_POLYGON)
    expect(polygon?.filled).toBe(true)
    expect(polygon?.paths[0]).toHaveLength(5)
    expect(parseGeometry('not json')).toBeUndefined()
    expect(parseGeometry('{"type":"Point","coordinates":"x"}')).toBeUndefined()
    expect(parseGeometry('  ')).toBeUndefined()
  })

  it('checks JSON, the type fitting the thing and the Swiss extent (as the server)', () => {
    const parcel = geometryRule(t, 'THING_SPECIALIZATION_PARCEL')
    expect(parcel(SAMPLE_POLYGON)).toBe(true)
    expect(parcel('')).toBe(true)
    expect(parcel('{')).toBe('validation.geometry.json')
    expect(parcel('{"type":"Point","coordinates":[2538000,1152000]}')).toBe('validation.geometry.type')
    expect(geometryRule(t)('{"type":"Point","coordinates":[6.63,46.52]}')).toBe('validation.geometry.extent')
  })

  it('projects north up into the viewBox', () => {
    const g = parseGeometry(SAMPLE_POLYGON)
    if (!g) {
      throw new Error('sample polygon must parse')
    }
    const svg = svgGeometry(g, 100, 10)
    expect(svg.spanMetres).toBe(40)
    expect(svg.paths[0]?.split(' ', 1)[0]).toBe('10.0,90.0')
  })

  it('links map.geo.admin.ch on a point', () => {
    expect(swissMapPointUrl(2_538_050.4, 1_152_600.6)).toContain('E=2538050&N=1152601')
    expect(swissMapPointUrl(undefined, 1)).toBeUndefined()
  })
})
