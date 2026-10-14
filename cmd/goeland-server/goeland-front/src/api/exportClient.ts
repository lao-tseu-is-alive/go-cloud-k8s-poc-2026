/**
 * Case export (GLD-029): a background job builds a ZIP of the case (manifest
 * plus every document version's bytes once); the requester downloads it until
 * it expires. Requesting needs MANAGE on the case and is recorded in the
 * sensitive read audit.
 */
import type { CaseExport, CaseExportResponse, ListCaseExportsResponse } from './types'
import { apiFetch, downloadAs } from './client'

const casePath = (id: string) => `/api/cases/${encodeURIComponent(id)}/exports`

/** Queues an export of a case. */
export async function requestCaseExport (caseId: string, reason: string): Promise<CaseExport | undefined> {
  const res = await apiFetch<CaseExportResponse>(casePath(caseId), { method: 'POST', body: { reason: reason || undefined } })
  return res.export
}

/** The caller's exports of a case, newest first. */
export async function listCaseExports (caseId: string): Promise<CaseExport[]> {
  const res = await apiFetch<ListCaseExportsResponse>(casePath(caseId))
  return res.exports ?? []
}

/** Saves a built archive. */
export function downloadCaseExport (exportId: string): Promise<void> {
  return downloadAs(`/api/case-exports/${encodeURIComponent(exportId)}/content`, `case-export-${exportId}.zip`)
}
