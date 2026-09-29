import type {
  CreateOrgUnitRequest,
  GetOrgUnitResponse,
  OrgUnit,
  OrgUnitInput,
  OrgUnitNode,
  OrgUnitType,
  SearchOrgUnitsParams,
  SearchOrgUnitsResponse,
} from './types'
/**
 * OrgUnitService REST calls. Reading needs goeland:read; every mutation needs
 * goeland:admin (the server is the authority).
 */
import { apiFetch } from './client'

const unitPath = (id: string) => `/api/org-units/${encodeURIComponent(id)}`

/** The whole tree as a flat list, ordered by type then label. */
export async function listOrgUnits (includeDissolved = false): Promise<OrgUnitNode[]> {
  const res = await apiFetch<{ units?: OrgUnitNode[] }>('/api/org-units', { query: { includeDissolved } })
  return res.units ?? []
}

export function searchOrgUnits (params: SearchOrgUnitsParams, signal?: AbortSignal): Promise<SearchOrgUnitsResponse> {
  return apiFetch<SearchOrgUnitsResponse>('/api/org-units/search', { query: params as Record<string, unknown>, signal })
}

export function getOrgUnit (
  id: string,
  opts: { includeRelationships?: boolean, includeAudit?: boolean } = {},
): Promise<GetOrgUnitResponse> {
  return apiFetch<GetOrgUnitResponse>(unitPath(id), {
    query: { includeRelationships: opts.includeRelationships, includeAudit: opts.includeAudit },
  })
}

export async function createOrgUnit (req: CreateOrgUnitRequest): Promise<OrgUnit> {
  const res = await apiFetch<{ orgUnit?: OrgUnit }>('/api/org-units', { method: 'POST', body: req })
  return res.orgUnit as OrgUnit
}

export async function updateOrgUnit (id: string, req: OrgUnitInput): Promise<OrgUnit> {
  const res = await apiFetch<{ orgUnit?: OrgUnit }>(unitPath(id), { method: 'PATCH', body: req })
  return res.orgUnit as OrgUnit
}

export async function dissolveOrgUnit (id: string, reason: string): Promise<OrgUnit> {
  const res = await apiFetch<{ orgUnit?: OrgUnit }>(`${unitPath(id)}/dissolve`, { method: 'POST', body: { reason } })
  return res.orgUnit as OrgUnit
}

export async function listOrgUnitTypes (onlyActive = true): Promise<OrgUnitType[]> {
  const res = await apiFetch<{ orgUnitTypes?: OrgUnitType[] }>('/api/org-unit-types', { query: { onlyActive } })
  return res.orgUnitTypes ?? []
}
