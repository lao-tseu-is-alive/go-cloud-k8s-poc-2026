/**
 * Retention of closed cases (GLD-032): the retention schedule, the retention
 * and legal hold of a case, the due list of the records manager and the
 * application of a case type's policy to its closed cases.
 */
import type {
  ApplyCaseTypeRetentionResponse, BordereauStatus, DecideDispositionItemResponse, DispositionBordereau, DispositionBordereauResponse,
  DispositionItem, FinalDisposition, GetCaseRetentionResponse, ListDispositionBordereauxResponse, ListDueRetentionsResponse,
  ListMyDispositionApprovalsResponse, ListRetentionPoliciesResponse, RetentionHold, RetentionHoldResponse, RetentionPolicy,
} from './types'
import { apiFetch, downloadAs } from './client'

/** The dispositions a policy may have. */
export const FINAL_DISPOSITIONS: readonly FinalDisposition[] = ['FINAL_DISPOSITION_ARCHIVE', 'FINAL_DISPOSITION_DESTROY', 'FINAL_DISPOSITION_REVIEW']

const holdPath = (caseId: string) => `/api/cases/${encodeURIComponent(caseId)}/retention-hold`

/** The retention policies, by label. */
export async function listRetentionPolicies (onlyActive: boolean): Promise<RetentionPolicy[]> {
  const res = await apiFetch<ListRetentionPoliciesResponse>('/api/retention-policies', { query: { onlyActive } })
  return res.retentionPolicies ?? []
}

/** The retention of a case: running and past retentions, holds and its type's policy. */
export function getCaseRetention (caseId: string): Promise<GetCaseRetentionResponse> {
  return apiFetch<GetCaseRetentionResponse>(`/api/cases/${encodeURIComponent(caseId)}/retention`)
}

/** Places a legal hold on a case (records manager). */
export async function placeRetentionHold (caseId: string, reason: string): Promise<RetentionHold | undefined> {
  return (await apiFetch<RetentionHoldResponse>(holdPath(caseId), { method: 'POST', body: { reason } })).hold
}

/** Releases the legal hold of a case (records manager). */
export async function releaseRetentionHold (caseId: string, reason: string): Promise<RetentionHold | undefined> {
  return (await apiFetch<RetentionHoldResponse>(`${holdPath(caseId)}/release`, { method: 'POST', body: { reason } })).hold
}

/** A page of the retentions ending on or before a day (today when dueOn is empty). */
export function listDueRetentions (params: {
  dueOn?: string
  finalDisposition?: FinalDisposition
  includeHeld?: boolean
  pageSize?: number
  pageToken?: string
} = {}): Promise<ListDueRetentionsResponse> {
  return apiFetch<ListDueRetentionsResponse>('/api/retentions/due', { query: params as Record<string, unknown> })
}

/** Gives a case type's policy to (at most 1000 of) its closed cases without retention (administrators). */
export function applyCaseTypeRetention (caseTypeCode: string, reason: string): Promise<ApplyCaseTypeRetentionResponse> {
  return apiFetch<ApplyCaseTypeRetentionResponse>(`/api/case-types/${encodeURIComponent(caseTypeCode)}/apply-retention`, { method: 'POST', body: { reason } })
}

const bordereauPath = (id: string) => `/api/disposition-bordereaux/${encodeURIComponent(id)}`

/** Prepares a bordereau of due cases (records manager). */
export async function createDispositionBordereau (finalDisposition: FinalDisposition, reason: string, caseIds: string[]): Promise<DispositionBordereau | undefined> {
  return (await apiFetch<DispositionBordereauResponse>('/api/disposition-bordereaux', { method: 'POST', body: { finalDisposition, reason, caseIds } })).bordereau
}

/** A page of bordereaux, newest first (records manager). */
export function listDispositionBordereaux (params: { status?: BordereauStatus, pageSize?: number, pageToken?: string } = {}): Promise<ListDispositionBordereauxResponse> {
  return apiFetch<ListDispositionBordereauxResponse>('/api/disposition-bordereaux', { query: params as Record<string, unknown> })
}

/** A bordereau with its cases (records manager). */
export async function getDispositionBordereau (id: string): Promise<DispositionBordereau | undefined> {
  return (await apiFetch<DispositionBordereauResponse>(bordereauPath(id))).bordereau
}

/** Executes a fully decided bordereau (records manager). */
export async function executeDispositionBordereau (id: string): Promise<DispositionBordereau | undefined> {
  return (await apiFetch<DispositionBordereauResponse>(`${bordereauPath(id)}/execute`, { method: 'POST', body: {} })).bordereau
}

/** Abandons an open bordereau (records manager). */
export async function cancelDispositionBordereau (id: string, reason: string): Promise<DispositionBordereau | undefined> {
  return (await apiFetch<DispositionBordereauResponse>(`${bordereauPath(id)}/cancel`, { method: 'POST', body: { reason } })).bordereau
}

/** The cases waiting for the caller's decision. */
export function listMyDispositionApprovals (): Promise<ListMyDispositionApprovalsResponse> {
  return apiFetch<ListMyDispositionApprovalsResponse>('/api/disposition-items/mine')
}

/** Approves or rejects a case of a bordereau. */
export async function decideDispositionItem (id: string, approve: boolean, reason: string): Promise<DispositionItem | undefined> {
  return (await apiFetch<DecideDispositionItemResponse>(`/api/disposition-items/${encodeURIComponent(id)}/decide`, { method: 'POST', body: { approve, reason } })).item
}

/** Saves a built transfer package (records manager). */
export function downloadTransferPackage (exportId: string): Promise<void> {
  return downloadAs(`/api/case-exports/${encodeURIComponent(exportId)}/content`, `transfer-${exportId}.zip`)
}
