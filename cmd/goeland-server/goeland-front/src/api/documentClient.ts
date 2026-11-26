import type {
  AddDocumentVersionRequest,
  CreateDocumentRequest,
  CreateDocumentResult,
  DocumentType,
  DocumentVersion,
  GetDocumentResponse,
  GoDocument,
  SearchDocumentsParams,
  SearchDocumentsResponse,
  SubjectRelationship,
  UpdateDocumentMetadataRequest, UploadResult,
  VerifyDocumentContentResponse,
} from './types'
/**
 * DocumentService REST calls + the (out-of-proto) multipart upload endpoint.
 * All business codes (document_type_code, relationship_type_code) are sent
 * as-is; never translated.
 */
import { apiFetch, apiFetchBlob, downloadAs } from './client'

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

export async function createDocument (req: CreateDocumentRequest): Promise<CreateDocumentResult> {
  const res = await apiFetch<{ document?: GoDocument, reused?: boolean }>('/api/documents', { method: 'POST', body: req })
  return { document: res.document as GoDocument, reused: !!res.reused }
}

export async function addDocumentVersion (documentId: string, req: AddDocumentVersionRequest): Promise<GoDocument> {
  const res = await apiFetch<{ document?: GoDocument }>(`/api/documents/${encodeURIComponent(documentId)}/versions`, {
    method: 'POST',
    body: req,
  })
  return res.document as GoDocument
}

export async function listDocumentVersions (documentId: string, signal?: AbortSignal): Promise<DocumentVersion[]> {
  const res = await apiFetch<{ versions?: DocumentVersion[] }>(`/api/documents/${encodeURIComponent(documentId)}/versions`, { signal })
  return res.versions ?? []
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

/** Rereads the stored bytes of a version (the current one by default) and records the result (GLD-021). */
export function verifyDocumentContent (id: string, versionId?: string): Promise<VerifyDocumentContentResponse> {
  return apiFetch<VerifyDocumentContentResponse>(`/api/documents/${encodeURIComponent(id)}/verify-content`, {
    method: 'POST',
    body: versionId ? { versionId } : {},
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
 * Uploads a file; the server stores it, computes its SHA-256 and registers it as
 * a (globally deduplicated) content blob whose id feeds createDocument() or
 * addDocumentVersion().
 * This hits the out-of-proto multipart endpoint added in the Go server.
 */
export function uploadDocumentFile (file: File, signal?: AbortSignal): Promise<UploadResult> {
  const form = new FormData()
  form.append('file', file)
  return apiFetch<UploadResult>('/api/documents/upload', { method: 'POST', formData: form, signal })
}

/**
 * Downloads the content of a document version, the current one by default (auth
 * and READ on the document required, so we fetch with the token and trigger a
 * browser save rather than using a plain <a href>).
 */
export function downloadDocumentBlob (documentId: string, filename: string, versionId?: string): Promise<void> {
  const query = versionId ? `?versionId=${encodeURIComponent(versionId)}` : ''
  return downloadAs(`/api/documents/${encodeURIComponent(documentId)}/content${query}`, filename || 'document')
}

/**
 * Reads the content of a document version, the current one by default, for an
 * inline preview (GLD-087): the same governed download (READ on the document,
 * DOWNLOAD recorded on a confidential one), fetched with the token.
 */
export function fetchDocumentContent (documentId: string, versionId?: string): Promise<Blob> {
  const query = versionId ? `?versionId=${encodeURIComponent(versionId)}` : ''
  return apiFetchBlob(`/api/documents/${encodeURIComponent(documentId)}/content${query}`)
}
