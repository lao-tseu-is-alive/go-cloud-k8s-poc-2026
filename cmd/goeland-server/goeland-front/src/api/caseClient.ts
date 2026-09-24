import type {
  CaseStatus,
  CaseType,
  CreateCaseRequest,
  GetCaseResponse,
  GoCase,
  SearchCasesParams,
  SearchCasesResponse,
  UpdateCaseRequest,
} from './types'
/**
 * CaseService REST calls. Business codes (case_type_code, statuses) are sent
 * as-is; never translated.
 */
import { apiFetch } from './client'

const casePath = (id: string) => `/api/cases/${encodeURIComponent(id)}`

export function searchCases (params: SearchCasesParams, signal?: AbortSignal): Promise<SearchCasesResponse> {
  return apiFetch<SearchCasesResponse>('/api/cases/search', { query: params as Record<string, unknown>, signal })
}

export function getCase (
  id: string,
  opts: { includeRelationships?: boolean, includeAudit?: boolean } = {},
): Promise<GetCaseResponse> {
  return apiFetch<GetCaseResponse>(casePath(id), {
    query: { includeRelationships: opts.includeRelationships, includeAudit: opts.includeAudit },
  })
}

export async function createCase (req: CreateCaseRequest): Promise<GoCase> {
  const res = await apiFetch<{ case?: GoCase }>('/api/cases', { method: 'POST', body: req })
  return res.case as GoCase
}

export async function updateCase (id: string, req: UpdateCaseRequest): Promise<GoCase> {
  const res = await apiFetch<{ case?: GoCase }>(casePath(id), { method: 'PATCH', body: req })
  return res.case as GoCase
}

export async function transitionCase (id: string, targetStatus: CaseStatus, reason?: string): Promise<GoCase> {
  const res = await apiFetch<{ case?: GoCase }>(`${casePath(id)}/transition`, {
    method: 'POST',
    body: { targetStatus, reason },
  })
  return res.case as GoCase
}

export function deleteCase (id: string, reason: string): Promise<unknown> {
  return apiFetch(casePath(id), { method: 'DELETE', query: { reason } })
}

export async function listCaseTypes (onlyActive = true): Promise<CaseType[]> {
  const res = await apiFetch<{ caseTypes?: CaseType[] }>('/api/case-types', { query: { onlyActive } })
  return res.caseTypes ?? []
}
