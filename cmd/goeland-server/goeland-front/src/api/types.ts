/**
 * TypeScript projections of the goeland.v1 proto messages as serialized by the
 * Vanguard REST transcoder (proto3 JSON):
 *   - field names are camelCase,
 *   - int64 fields are serialized as strings (e.g. fileSizeBytes: "38"),
 *   - enums are their string names (e.g. "DOCUMENT_STATUS_DRAFT"),
 *   - Timestamps are RFC3339 strings,
 *   - google.protobuf.Struct (metadata) is a plain JSON object.
 *
 * These are hand-maintained (no codegen in this POC) and only cover the fields
 * the UI actually consumes.
 */

export type SubjectKind
  = | 'SUBJECT_KIND_UNSPECIFIED'
    | 'SUBJECT_KIND_CASE'
    | 'SUBJECT_KIND_DOCUMENT'
    | 'SUBJECT_KIND_THING'
    | 'SUBJECT_KIND_ACTOR'
    | 'SUBJECT_KIND_USER'
    | 'SUBJECT_KIND_ORG_UNIT'

export type Permission
  = | 'PERMISSION_UNSPECIFIED'
    | 'PERMISSION_NONE'
    | 'PERMISSION_READ'
    | 'PERMISSION_CONTRIBUTE'
    | 'PERMISSION_MANAGE'
    | 'PERMISSION_FULL_CONTROL'

export type DocumentStatus
  = | 'DOCUMENT_STATUS_UNSPECIFIED'
    | 'DOCUMENT_STATUS_DRAFT'
    | 'DOCUMENT_STATUS_FINAL'
    | 'DOCUMENT_STATUS_SUPERSEDED'
    | 'DOCUMENT_STATUS_ARCHIVED'

export interface SubjectRef {
  id: string
  kind: SubjectKind
  displayLabel: string
  canonicalUrl?: string
  createdAt?: string
}

export interface RecordMetadata {
  createdAt?: string
  createdBy?: string
  updatedAt?: string
  updatedBy?: string
  deletedAt?: string
  deletedBy?: string
  ownerUserId?: string
  ownerOrgId?: string
  confidentialityLevel?: number
  version?: number
  isLocked?: boolean
  lockedAt?: string
  lockedBy?: string
  retentionUntil?: string
  sortFinal?: string
}

export interface DocumentType {
  id: string
  code: string
  label: string
  description?: string
  category?: string
  isActive?: boolean
}

export interface GoDocument {
  subjectRef?: SubjectRef
  documentType?: DocumentType
  title: string
  description?: string
  officialDate?: string
  storageRef?: string
  externalSystem?: string
  externalId?: string
  externalUrl?: string
  mimeType?: string
  fileSizeBytes?: string // int64 as string
  sha256?: string
  sha256VerifiedAt?: string
  version?: number
  previousVersionId?: string
  isFinal?: boolean
  isRecord?: boolean
  language?: string
  pageCount?: number
  status?: DocumentStatus
  metadata?: Record<string, unknown>
  createdAt?: string
  createdBy?: string
  updatedAt?: string
  recordMetadata?: RecordMetadata
}

export interface RelationshipType {
  id?: string
  code: string
  label: string
  sourceKind?: SubjectKind
  targetKind?: SubjectKind
  isDirected?: boolean
  inverseLabel?: string
  isActive?: boolean
}

export interface SubjectRelationship {
  id: string
  source?: SubjectRef
  target?: SubjectRef
  relationshipType?: RelationshipType
  roleDetail?: string
  validFrom?: string
  validTo?: string
  createdAt?: string
  createdBy?: string
  deletedAt?: string
}

export interface AuditEvent {
  id?: string
  subjectId?: string
  eventType?: string
  actorUserId?: string
  occurredAt?: string
  beforeState?: Record<string, unknown>
  afterState?: Record<string, unknown>
  reason?: string
  correlationId?: string
  requestId?: string
}

// ---- request/response shapes ---------------------------------------------

export interface SearchDocumentsParams {
  query?: string
  documentTypeCode?: string
  caseId?: string
  thingId?: string
  confidentialityMax?: number
  onlyRecords?: boolean
  onlyFinal?: boolean
  includeDeleted?: boolean
  pageSize?: number
  pageToken?: string
}

export interface SearchDocumentsResponse {
  documents?: GoDocument[]
  nextPageToken?: string
  totalSize?: number
}

export interface GetDocumentResponse {
  document?: GoDocument
  relationships?: SubjectRelationship[]
  recentAudit?: AuditEvent[]
}

export interface CreateDocumentRequest {
  documentTypeCode: string
  title: string
  description?: string
  officialDate?: string
  storageRef?: string
  externalSystem?: string
  externalId?: string
  externalUrl?: string
  mimeType?: string
  fileSizeBytes?: string
  sha256?: string
  version?: number
  isFinal?: boolean
  isRecord?: boolean
  language?: string
  pageCount?: number
  metadata?: Record<string, unknown>
  linkToCaseId?: string
}

export interface UpdateDocumentMetadataRequest {
  title?: string
  description?: string
  officialDate?: string
  language?: string
  metadata?: Record<string, unknown>
  reason?: string
}

export interface UploadResult {
  storageRef: string
  sha256: string
  fileSizeBytes: string
  mimeType: string
  filename: string
}

/** Payload returned by GET /config. */
export interface FrontendConfig {
  authMode: 'dev' | 'jwt'
  authBaseUrl: string
}

/** Authenticated user shape from the auth server's silent token mint. */
export interface TokenUser {
  user_id?: number
  name?: string
  email?: string
  is_admin?: boolean
}

export interface TokenResponse {
  token: string
  user: TokenUser
  expires_in_seconds: number
}
