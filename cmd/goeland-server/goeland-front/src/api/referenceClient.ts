/**
 * Reference data administration (GLD-040): create / update calls of the four
 * catalogues (administrators only) and the reference change log. Listing reuses
 * each domain client (listCaseTypes, listRelationshipTypes, ...).
 */
import type { ListReferenceChangesResponse, ReferenceCatalogue, ReferenceChange } from './types'
import { apiFetch } from './client'

/** REST collection of each catalogue. */
export const CATALOGUE_PATHS: Record<ReferenceCatalogue, string> = {
  case_type: '/api/case-types',
  relationship_type: '/api/relationship-types',
  organization_category: '/api/organization-categories',
  document_type: '/api/document-types',
}

/** Response field holding the entry, per catalogue. */
const ENTRY_KEYS: Record<ReferenceCatalogue, string> = {
  case_type: 'caseType',
  relationship_type: 'relationshipType',
  organization_category: 'organizationCategory',
  document_type: 'documentType',
}

type EntryResponse = Record<string, unknown> & { change?: ReferenceChange }

/** Creates a catalogue entry; body holds code, label and the catalogue's fields. */
export async function createReferenceEntry (catalogue: ReferenceCatalogue, body: Record<string, unknown>): Promise<unknown> {
  const res = await apiFetch<EntryResponse>(CATALOGUE_PATHS[catalogue], { method: 'POST', body })
  return res[ENTRY_KEYS[catalogue]]
}

/** Updates a catalogue entry by code; absent fields are kept. */
export async function updateReferenceEntry (catalogue: ReferenceCatalogue, code: string, body: Record<string, unknown>): Promise<unknown> {
  const res = await apiFetch<EntryResponse>(`${CATALOGUE_PATHS[catalogue]}/${encodeURIComponent(code)}`, { method: 'PATCH', body })
  return res[ENTRY_KEYS[catalogue]]
}

/** A page of the reference change log, newest first. */
export function listReferenceChanges (
  params: { catalogue?: ReferenceCatalogue, pageSize?: number, pageToken?: string } = {},
): Promise<ListReferenceChangesResponse> {
  return apiFetch<ListReferenceChangesResponse>('/api/reference-changes', { query: params as Record<string, unknown> })
}
