/**
 * ThingService REST calls (GLD-016): things (parcels, buildings, ...) and their types.
 */
import type {
  CreateThingRequest,
  GetThingResponse,
  GoThing,
  SearchThingsParams,
  SearchThingsResponse,
  ThingType,
  UpdateThingRequest,
} from './types'
import { apiFetch } from './client'

function thingPath (id: string): string {
  return `/api/things/${encodeURIComponent(id)}`
}

export function searchThings (params: SearchThingsParams, signal?: AbortSignal): Promise<SearchThingsResponse> {
  return apiFetch<SearchThingsResponse>('/api/things/search', { query: params as Record<string, unknown>, signal })
}

export function getThing (id: string, opts: { includeRelationships?: boolean, includeAudit?: boolean } = {}): Promise<GetThingResponse> {
  return apiFetch<GetThingResponse>(thingPath(id), {
    query: { includeRelationships: opts.includeRelationships, includeAudit: opts.includeAudit },
  })
}

export async function createThing (req: CreateThingRequest): Promise<GoThing> {
  const res = await apiFetch<{ thing?: GoThing }>('/api/things', { method: 'POST', body: req })
  return res.thing as GoThing
}

export async function updateThing (id: string, req: UpdateThingRequest): Promise<GoThing> {
  const res = await apiFetch<{ thing?: GoThing }>(thingPath(id), { method: 'PATCH', body: req })
  return res.thing as GoThing
}

export function deleteThing (id: string, reason: string): Promise<unknown> {
  return apiFetch(thingPath(id), { method: 'DELETE', query: { reason } })
}

export async function listThingTypes (onlyActive = true): Promise<ThingType[]> {
  const res = await apiFetch<{ thingTypes?: ThingType[] }>('/api/thing-types', { query: { onlyActive } })
  return res.thingTypes ?? []
}
