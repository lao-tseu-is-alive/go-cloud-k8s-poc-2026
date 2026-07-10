import type {
  CreateActorRequest,
  GetActorResponse,
  GoActor,
  OrganizationCategory,
  SearchActorsParams,
  SearchActorsResponse,
  UpdateActorRequest,
} from './types'
/**
 * ActorService REST calls. All business codes (organizationCategoryCode,
 * relationship_type_code) are sent as-is; never translated. Enum values travel
 * as their proto3-JSON string names (e.g. "ACTOR_KIND_ORGANIZATION").
 */
import { apiFetch } from './client'

export function searchActors (params: SearchActorsParams, signal?: AbortSignal): Promise<SearchActorsResponse> {
  return apiFetch<SearchActorsResponse>('/api/actors/search', {
    query: params as Record<string, unknown>,
    signal,
  })
}

export function getActor (
  id: string,
  opts: { includeRelationships?: boolean, includeAudit?: boolean } = {},
  signal?: AbortSignal,
): Promise<GetActorResponse> {
  return apiFetch<GetActorResponse>(`/api/actors/${encodeURIComponent(id)}`, {
    query: { includeRelationships: opts.includeRelationships, includeAudit: opts.includeAudit },
    signal,
  })
}

export async function createActor (req: CreateActorRequest): Promise<GoActor> {
  const res = await apiFetch<{ actor?: GoActor }>('/api/actors', { method: 'POST', body: req })
  return res.actor as GoActor
}

export async function updateActor (id: string, req: UpdateActorRequest): Promise<GoActor> {
  const res = await apiFetch<{ actor?: GoActor }>(`/api/actors/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: req,
  })
  return res.actor as GoActor
}

export function deleteActor (id: string, reason: string): Promise<unknown> {
  return apiFetch(`/api/actors/${encodeURIComponent(id)}`, { method: 'DELETE', query: { reason } })
}

export async function listOrganizationCategories (onlyActive = true): Promise<OrganizationCategory[]> {
  const res = await apiFetch<{ categories?: OrganizationCategory[] }>('/api/organization-categories', {
    query: { onlyActive },
  })
  return res.categories ?? []
}
