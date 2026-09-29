import type {
  CreateTimelineEntryRequest,
  ListTimelineParams,
  ListTimelineResponse,
  TimelineEntry,
  UpdateTimelineEntryRequest,
} from './types'
/**
 * TimelineService REST calls (case timeline, "suivis"). Every mutation returns
 * the entry as stored, with its documents.
 */
import { apiFetch } from './client'

const timelinePath = (caseId: string) => `/api/cases/${encodeURIComponent(caseId)}/timeline`
const entryPath = (id: string) => `/api/timeline-entries/${encodeURIComponent(id)}`

type EntryResponse = { entry?: TimelineEntry }

export function listTimeline (caseId: string, params: ListTimelineParams = {}): Promise<ListTimelineResponse> {
  return apiFetch<ListTimelineResponse>(timelinePath(caseId), { query: params as Record<string, unknown> })
}

export async function createTimelineEntry (caseId: string, req: CreateTimelineEntryRequest): Promise<TimelineEntry> {
  const res = await apiFetch<EntryResponse>(timelinePath(caseId), { method: 'POST', body: req })
  return res.entry as TimelineEntry
}

export async function updateTimelineEntry (id: string, req: UpdateTimelineEntryRequest): Promise<TimelineEntry> {
  const res = await apiFetch<EntryResponse>(entryPath(id), { method: 'PATCH', body: req })
  return res.entry as TimelineEntry
}

/** Moves a draft to an immutable state: validate (endorse), lock (freeze as is) or withdraw (set aside). */
export async function changeTimelineEntryStatus (
  id: string,
  action: 'validate' | 'lock' | 'withdraw',
  reason?: string,
): Promise<TimelineEntry> {
  const res = await apiFetch<EntryResponse>(`${entryPath(id)}/${action}`, { method: 'POST', body: { reason } })
  return res.entry as TimelineEntry
}

export async function linkTimelineDocument (entryId: string, documentId: string): Promise<TimelineEntry> {
  const res = await apiFetch<EntryResponse>(`${entryPath(entryId)}/documents`, { method: 'POST', body: { documentId } })
  return res.entry as TimelineEntry
}

export async function unlinkTimelineDocument (entryId: string, documentId: string, reason?: string): Promise<TimelineEntry> {
  const res = await apiFetch<EntryResponse>(`${entryPath(entryId)}/documents/${encodeURIComponent(documentId)}`, {
    method: 'DELETE',
    query: { reason },
  })
  return res.entry as TimelineEntry
}
