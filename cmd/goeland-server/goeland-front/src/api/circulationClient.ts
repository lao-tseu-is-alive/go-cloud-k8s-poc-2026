import type { Circulation, CirculationResponse, CreateCirculationRequest } from './types'
/**
 * CirculationService REST calls. Every mutation returns the circulation as
 * stored (with its recipients and their tasks).
 */
import { apiFetch } from './client'

type CirculationResponseBody = { circulation?: Circulation }

export async function listCaseCirculations (caseId: string): Promise<Circulation[]> {
  const res = await apiFetch<{ circulations?: Circulation[] }>(`/api/cases/${encodeURIComponent(caseId)}/circulations`)
  return res.circulations ?? []
}

export async function getCirculation (id: string): Promise<Circulation> {
  const res = await apiFetch<CirculationResponseBody>(`/api/circulations/${encodeURIComponent(id)}`)
  return res.circulation as Circulation
}

export async function createCirculation (caseId: string, req: CreateCirculationRequest): Promise<Circulation> {
  const res = await apiFetch<CirculationResponseBody>(`/api/cases/${encodeURIComponent(caseId)}/circulations`, { method: 'POST', body: req })
  return res.circulation as Circulation
}

export async function respondToCirculation (recipientId: string, response: CirculationResponse, text?: string): Promise<Circulation> {
  const res = await apiFetch<CirculationResponseBody>(`/api/circulation-recipients/${encodeURIComponent(recipientId)}/respond`, {
    method: 'POST',
    body: { response, text },
  })
  return res.circulation as Circulation
}

export async function cancelCirculation (id: string, reason: string): Promise<Circulation> {
  const res = await apiFetch<CirculationResponseBody>(`/api/circulations/${encodeURIComponent(id)}/cancel`, { method: 'POST', body: { reason } })
  return res.circulation as Circulation
}
