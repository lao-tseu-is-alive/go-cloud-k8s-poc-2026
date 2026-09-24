/**
 * Flat form model of a thing and its mapping to the proto3-JSON requests; only
 * the detail block matching the type's specialization is sent.
 */
import type {
  BuildingStatus,
  CreateThingRequest,
  GoThing,
  ThingSpecialization,
  UpdateThingRequest,
} from '@/api/types'

export interface ThingFormModel {
  thingTypeCode: string | undefined
  specialization: ThingSpecialization
  name: string
  description: string
  externalRef: string
  geometry: string
  communeOfs: number | undefined
  parcelNumber: string
  egrid: string
  surfaceM2: number | undefined
  egid: number | undefined
  ecaNumber: string
  constructionYear: number | undefined
  buildingStatus: BuildingStatus
}

export const BUILDING_STATUSES: BuildingStatus[] = [
  'BUILDING_STATUS_UNSPECIFIED', 'BUILDING_STATUS_PLANNED', 'BUILDING_STATUS_AUTHORIZED', 'BUILDING_STATUS_UNDER_CONSTRUCTION',
  'BUILDING_STATUS_EXISTING', 'BUILDING_STATUS_UNUSABLE', 'BUILDING_STATUS_DEMOLISHED', 'BUILDING_STATUS_NOT_REALIZED',
]

/** Lausanne's OFS commune number, the default of a new parcel. */
export const LAUSANNE_OFS = 5586

export function emptyThingForm (): ThingFormModel {
  return {
    thingTypeCode: undefined,
    specialization: 'THING_SPECIALIZATION_UNSPECIFIED',
    name: '',
    description: '',
    externalRef: '',
    geometry: '',
    communeOfs: LAUSANNE_OFS,
    parcelNumber: '',
    egrid: '',
    surfaceM2: undefined,
    egid: undefined,
    ecaNumber: '',
    constructionYear: undefined,
    buildingStatus: 'BUILDING_STATUS_UNSPECIFIED',
  }
}

export function thingToForm (thing: GoThing): ThingFormModel {
  return {
    thingTypeCode: thing.thingType?.code,
    specialization: thing.thingType?.specialization ?? 'THING_SPECIALIZATION_UNSPECIFIED',
    name: thing.name,
    description: thing.description ?? '',
    externalRef: thing.externalRef ?? '',
    geometry: thing.geometryGeojson ?? '',
    communeOfs: thing.parcel?.communeOfs ?? LAUSANNE_OFS,
    parcelNumber: thing.parcel?.parcelNumber ?? '',
    egrid: thing.parcel?.egrid ?? '',
    surfaceM2: thing.parcel?.surfaceM2,
    egid: thing.building?.egid,
    ecaNumber: thing.building?.ecaNumber ?? '',
    constructionYear: thing.building?.constructionYear,
    buildingStatus: thing.building?.buildingStatus ?? 'BUILDING_STATUS_UNSPECIFIED',
  }
}

/** A number typed in a field (v-text-field type=number gives strings or numbers). */
function num (v: unknown): number | undefined {
  const n = typeof v === 'number' ? v : Number(v)
  return typeof v === 'string' && v.trim() === '' ? undefined : (Number.isFinite(n) && v !== undefined ? n : undefined)
}

function details (m: ThingFormModel): Pick<CreateThingRequest, 'parcel' | 'building'> {
  if (m.specialization === 'THING_SPECIALIZATION_PARCEL') {
    return { parcel: { communeOfs: num(m.communeOfs) ?? 0, parcelNumber: m.parcelNumber.trim(), egrid: m.egrid.trim().toUpperCase() || undefined, surfaceM2: num(m.surfaceM2) } }
  }
  if (m.specialization === 'THING_SPECIALIZATION_BUILDING') {
    return { building: { egid: num(m.egid), ecaNumber: m.ecaNumber.trim() || undefined, constructionYear: num(m.constructionYear), buildingStatus: m.buildingStatus } }
  }
  return {}
}

export function buildCreateThingRequest (m: ThingFormModel): CreateThingRequest {
  return {
    thingTypeCode: m.thingTypeCode ?? '',
    name: m.name.trim() || undefined,
    description: m.description.trim() || undefined,
    externalRef: m.externalRef.trim() || undefined,
    geometryGeojson: m.geometry.trim() || undefined,
    ...details(m),
  }
}

export function buildUpdateThingRequest (m: ThingFormModel, reason: string): UpdateThingRequest {
  return {
    name: m.name.trim(),
    description: m.description.trim(),
    externalRef: m.externalRef.trim(),
    geometryGeojson: m.geometry.trim(),
    ...details(m),
    reason: reason.trim() || undefined,
  }
}
