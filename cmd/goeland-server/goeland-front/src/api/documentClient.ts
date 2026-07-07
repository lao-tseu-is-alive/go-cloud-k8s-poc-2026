import type {
  CreateDocumentRequest,
  DocumentType,
  GetDocumentResponse,
  GoDocument,
  SearchDocumentsParams,
  SearchDocumentsResponse,
  SubjectRelationship,
  UpdateDocumentMetadataRequest, UploadResult,
} from './types'
/**
 * DocumentService REST calls + the (out-of-proto) multipart upload endpoint.
 * All business codes (document_type_code, relationship_type_code) are sent
 * as-is; never translated.
 */
import { apiFetch, apiFetchBlob } from './client'

export function searchDocuments (params: SearchDocumentsParams, signal?: AbortSignal): Promise<SearchDocumentsResponse> {
  return apiFetch<SearchDocumentsResponse>('/api/documents/search', {
    query: params as Record<string, unknown>,
    signal,
  })
}

export function getDocument (
  id: string,
  opts: { includeRelationships?: boolean, includeAudit?: boolean } = {},
  signal?: AbortSignal,
): Promise<GetDocumentResponse> {
  return apiFetch<GetDocumentResponse>(`/api/documents/${encodeURIComponent(id)}`, {
    query: { includeRelationships: opts.includeRelationships, includeAudit: opts.includeAudit },
    signal,
  })
}

export async function createDocument (req: CreateDocumentRequest): Promise<GoDocument> {
  const res = await apiFetch<{ document?: GoDocument }>('/api/documents', { method: 'POST', body: req })
  return res.document as GoDocument
}

export async function updateDocumentMetadata (id: string, req: UpdateDocumentMetadataRequest): Promise<GoDocument> {
  const res = await apiFetch<{ document?: GoDocument }>(`/api/documents/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: req,
  })
  return res.document as GoDocument
}

export async function finalizeDocument (id: string, reason: string, alsoLockGovernance: boolean): Promise<GoDocument> {
  const res = await apiFetch<{ document?: GoDocument }>(`/api/documents/${encodeURIComponent(id)}/finalize`, {
    method: 'POST',
    body: { reason, alsoLockGovernance },
  })
  return res.document as GoDocument
}

export interface IntegrityResult {
  verified?: boolean
  actualSha256?: string
  verifiedAt?: string
  storageRefChecked?: string
}

export function verifyDocumentIntegrity (id: string, expectedSha256?: string): Promise<IntegrityResult> {
  return apiFetch<IntegrityResult>(`/api/documents/${encodeURIComponent(id)}/integrity`, {
    query: { expectedSha256 },
  })
}

export async function linkDocument (
  documentId: string,
  targetSubjectId: string,
  relationshipTypeCode: string,
  roleDetail?: string,
): Promise<SubjectRelationship> {
  const res = await apiFetch<{ relationship?: SubjectRelationship }>(
    `/api/documents/${encodeURIComponent(documentId)}/links`,
    { method: 'POST', body: { targetSubjectId, relationshipTypeCode, roleDetail } },
  )
  return res.relationship as SubjectRelationship
}

export function deleteDocument (id: string, reason: string): Promise<unknown> {
  return apiFetch(`/api/documents/${encodeURIComponent(id)}`, { method: 'DELETE', query: { reason } })
}

export async function listDocumentTypes (onlyActive = true): Promise<DocumentType[]> {
  const res = await apiFetch<{ documentTypes?: DocumentType[] }>('/api/document-types', {
    query: { onlyActive },
  })
  return res.documentTypes ?? []
}

/**
 * Uploads a file to the internal blob store and returns the storage_ref plus
 * server-computed integrity metadata to feed into createDocument().
 * This hits the out-of-proto multipart endpoint added in the Go server.
 */
export function uploadDocumentFile (file: File, signal?: AbortSignal): Promise<UploadResult> {
  const form = new FormData()
  form.append('file', file)
  return apiFetch<UploadResult>('/api/documents/upload', { method: 'POST', formData: form, signal })
}

/**
 * Downloads a stored blob (auth required, so we fetch with the token and trigger
 * a browser save rather than using a plain <a href>).
 */
export async function downloadDocumentBlob (storageRef: string, filename: string): Promise<void> {
  const blob = await apiFetchBlob(`/api/documents/download?ref=${encodeURIComponent(storageRef)}`)
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename || 'document'
  document.body.append(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}
