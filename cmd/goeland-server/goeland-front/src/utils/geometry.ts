import type { Rule } from './validation'
/**
 * GeoJSON helpers for things (EPSG:2056, LV95 metres), mirroring the server's
 * checks (pkg/thing/geometry.go) for immediate feedback, plus what the SVG
 * preview needs.
 */
import type { ThingSpecialization } from '@/api/types'
import type { useI18n } from 'vue-i18n'

type TFn = ReturnType<typeof useI18n>['t']
type Position = [number, number]

/** Generous LV95 envelope of Switzerland (as the server). */
const SWISS = { eMin: 2_480_000, nMin: 1_070_000, eMax: 2_840_000, nMax: 1_300_000 }

const ALLOWED: Partial<Record<ThingSpecialization, string[]>> = {
  THING_SPECIALIZATION_PARCEL: ['Polygon', 'MultiPolygon'],
  THING_SPECIALIZATION_BUILDING: ['Point', 'Polygon', 'MultiPolygon'],
}

export interface ParsedGeometry {
  type: string
  /** Point positions. */
  points: Position[]
  /** Line and ring paths. */
  paths: Position[][]
  /** Whether paths are polygon rings (filled). */
  filled: boolean
}

function isPosition (v: unknown): v is Position {
  return Array.isArray(v) && v.length >= 2 && typeof v[0] === 'number' && typeof v[1] === 'number'
}

/** Flattens nested coordinate arrays into paths of positions. */
function collectPaths (coords: unknown, out: Position[][]): void {
  if (!Array.isArray(coords)) {
    return
  }
  if (coords.every(p => isPosition(p))) {
    out.push(coords.map(p => [p[0], p[1]]))
    return
  }
  for (const c of coords) {
    collectPaths(c, out)
  }
}

/** Parses a GeoJSON geometry object, or returns undefined when it is not one. */
export function parseGeometry (geojson?: string): ParsedGeometry | undefined {
  if (!geojson?.trim()) {
    return undefined
  }
  let value: unknown
  try {
    value = JSON.parse(geojson)
  } catch {
    return undefined
  }
  if (typeof value !== 'object' || value === null) {
    return undefined
  }
  const { type, coordinates } = value as { type?: unknown, coordinates?: unknown }
  if (typeof type !== 'string') {
    return undefined
  }
  if (type === 'Point') {
    return isPosition(coordinates) ? { type, points: [coordinates], paths: [], filled: false } : undefined
  }
  if (type === 'MultiPoint' && Array.isArray(coordinates)) {
    return { type, points: coordinates.filter(p => isPosition(p)), paths: [], filled: false }
  }
  const paths: Position[][] = []
  collectPaths(coordinates, paths)
  return paths.length > 0 ? { type, points: [], paths, filled: type.includes('Polygon') } : undefined
}

function allPositions (g: ParsedGeometry): Position[] {
  return [...g.points, ...g.paths.flat()]
}

/** Vuetify rule: parseable GeoJSON, allowed type, LV95 coordinates inside Switzerland. */
export function geometryRule (t: TFn, specialization?: ThingSpecialization): Rule {
  return v => {
    const s = typeof v === 'string' ? v.trim() : ''
    if (s === '') {
      return true
    }
    const g = parseGeometry(s)
    if (!g) {
      return t('validation.geometry.json')
    }
    const allowed = specialization ? ALLOWED[specialization] : undefined
    if (allowed && !allowed.includes(g.type)) {
      return t('validation.geometry.type', { allowed: allowed.join(', ') })
    }
    const inside = allPositions(g).every(([e, n]) => e >= SWISS.eMin && e <= SWISS.eMax && n >= SWISS.nMin && n <= SWISS.nMax)
    return inside || t('validation.geometry.extent')
  }
}

/** Projects a geometry into an SVG viewBox (north up) of the given size with a margin. */
export function svgGeometry (g: ParsedGeometry, size: number, margin = 8) {
  const pos = allPositions(g)
  const es = pos.map(p => p[0])
  const ns = pos.map(p => p[1])
  const eMin = Math.min(...es)
  const nMax = Math.max(...ns)
  const span = Math.max(Math.max(...es) - eMin, nMax - Math.min(...ns), 1)
  const scale = (size - 2 * margin) / span
  const project = ([e, n]: Position) => `${(margin + (e - eMin) * scale).toFixed(1)},${(margin + (nMax - n) * scale).toFixed(1)}`
  return {
    paths: g.paths.map(path => path.map(p => project(p)).join(' ')),
    points: g.points.map(p => project(p).split(',').map(Number) as [number, number]),
    spanMetres: span,
  }
}

/** A map.geo.admin.ch link centred on an LV95 point. */
export function swissMapPointUrl (e?: number, n?: number): string | undefined {
  if (!e || !n) {
    return undefined
  }
  return `https://map.geo.admin.ch/?E=${Math.round(e)}&N=${Math.round(n)}&zoom=10&crosshair=marker`
}

/** A sample LV95 square near Lausanne's Place de la Palud, to show the expected format. */
export const SAMPLE_POLYGON = JSON.stringify({
  type: 'Polygon',
  coordinates: [[[2_538_050, 1_152_600], [2_538_090, 1_152_600], [2_538_090, 1_152_640], [2_538_050, 1_152_640], [2_538_050, 1_152_600]]],
})
