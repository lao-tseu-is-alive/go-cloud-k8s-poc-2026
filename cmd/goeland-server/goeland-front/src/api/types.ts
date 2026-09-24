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
  /** Human business reference, e.g. "2026-001245"; absent/empty when none. */
  businessRef?: string
  /** Namespace scoping businessRef (e.g. "OPC"); empty for a free reference. */
  businessRefNamespace?: string
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

/** Binary content identified by its SHA-256 (stored once, shared by versions). */
export interface ContentBlob {
  id: string
  sha256?: string
  /** internal://… ref; empty when only the digest is known (bytes held elsewhere). */
  storageRef?: string
  mimeType?: string
  fileSizeBytes?: string // int64 as string
  createdAt?: string
  createdBy?: string
  verifiedAt?: string
}

/** A dated, append-only state of a document; final/record versions are immutable. */
export interface DocumentVersion {
  id: string
  documentId?: string
  versionNo?: number
  /** Absent for a metadata-only version or an external reference. */
  content?: ContentBlob
  pageCount?: number
  isFinal?: boolean
  isRecord?: boolean
  validatedAt?: string
  validatedBy?: string
  metadata?: Record<string, unknown>
  createdAt?: string
  createdBy?: string
}

export interface GoDocument {
  subjectRef?: SubjectRef
  documentType?: DocumentType
  title: string
  description?: string
  officialDate?: string
  externalSystem?: string
  externalId?: string
  externalUrl?: string
  language?: string
  status?: DocumentStatus
  metadata?: Record<string, unknown>
  createdAt?: string
  createdBy?: string
  updatedAt?: string
  recordMetadata?: RecordMetadata
  /** The explicit current version, with its content. */
  currentVersion?: DocumentVersion
}

// ---- actor ----------------------------------------------------------------

export type ActorKind
  = | 'ACTOR_KIND_UNSPECIFIED'
    | 'ACTOR_KIND_PERSON'
    | 'ACTOR_KIND_ORGANIZATION'

export type ContactType
  = | 'CONTACT_TYPE_UNSPECIFIED'
    | 'CONTACT_TYPE_PHONE'
    | 'CONTACT_TYPE_PHONE_PRIVATE'
    | 'CONTACT_TYPE_PHONE_PRO'
    | 'CONTACT_TYPE_MOBILE'
    | 'CONTACT_TYPE_FAX'
    | 'CONTACT_TYPE_EMAIL'
    | 'CONTACT_TYPE_WEBSITE'
    | 'CONTACT_TYPE_POSTAL_BOX'
    | 'CONTACT_TYPE_IDE_FEDERAL'
    | 'CONTACT_TYPE_VAT_NUMBER'
    | 'CONTACT_TYPE_ABACUS_DEBTOR'
    | 'CONTACT_TYPE_COMMERCIAL_REGISTER'
    | 'CONTACT_TYPE_OTHER'

export interface OrganizationCategory {
  id?: string
  code: string
  label: string
  isActive?: boolean
}

export interface ActorContact {
  contactType: ContactType
  value: string
  isPrimary?: boolean
  label?: string
}

export interface OrganizationDetails {
  legalName: string
  categoryCode?: string
  complement?: string
}

/** Form of address of a PERSON actor. */
export type Salutation
  = | 'SALUTATION_UNSPECIFIED'
    | 'SALUTATION_MADAME'
    | 'SALUTATION_MONSIEUR'
    | 'SALUTATION_NEUTRAL'

export interface PersonDetails {
  isChRegister?: boolean
  chRegisterRef?: string
  salutation?: Salutation
  lastName?: string
  firstName?: string
}

/** Role of an address for one actor. */
export type AddressType
  = | 'ADDRESS_TYPE_UNSPECIFIED'
    | 'ADDRESS_TYPE_HEAD_OFFICE'
    | 'ADDRESS_TYPE_BRANCH'
    | 'ADDRESS_TYPE_CORRESPONDENCE'
    | 'ADDRESS_TYPE_BILLING'
    | 'ADDRESS_TYPE_RESIDENCE'
    | 'ADDRESS_TYPE_OTHER'

/** A postal address of an actor (the link id is output-only). */
export interface ActorAddress {
  id?: string
  addressType: AddressType
  isPrincipal?: boolean
  street: string
  houseNumber?: string
  addressLine2?: string
  postalCode: string
  locality: string
  countryCode?: string
  label?: string
  createdAt?: string
}

export interface GoActor {
  subjectRef?: SubjectRef
  actorKind: ActorKind
  displayName: string
  nameForSearch?: string
  isActive?: boolean
  publicationCode?: number
  person?: PersonDetails
  organization?: OrganizationDetails
  contacts?: ActorContact[]
  addresses?: ActorAddress[]
  createdAt?: string
  createdBy?: string
  updatedAt?: string
  recordMetadata?: RecordMetadata
}

export interface SearchActorsParams {
  query?: string
  actorKind?: ActorKind
  organizationCategoryCode?: string
  onlyActive?: boolean
  includeDeleted?: boolean
  pageSize?: number
  pageToken?: string
}

export interface SearchActorsResponse {
  actors?: GoActor[]
  nextPageToken?: string
  totalSize?: number
}

export interface GetActorResponse {
  actor?: GoActor
  relationships?: SubjectRelationship[]
  recentAudit?: AuditEvent[]
}

export interface CreateActorRequest {
  actorKind: ActorKind
  displayName: string
  publicationCode?: number
  person?: PersonDetails
  organization?: OrganizationDetails
  contacts?: ActorContact[]
  addresses?: ActorAddress[]
}

export interface UpdateActorRequest {
  displayName?: string
  isActive?: boolean
  publicationCode?: number
  person?: PersonDetails
  organization?: OrganizationDetails
  replaceContacts?: boolean
  contacts?: ActorContact[]
  replaceAddresses?: boolean
  addresses?: ActorAddress[]
  reason?: string
}

export interface RelationshipType {
  id?: string
  code: string
  label: string
  sourceKind?: SubjectKind
  targetKind?: SubjectKind
  isDirected?: boolean
  inverseLabel?: string
  description?: string
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
  externalSystem?: string
  externalId?: string
  externalUrl?: string
  /** Content registered by the upload endpoint (UploadResult.contentBlobId). */
  contentBlobId?: string
  isFinal?: boolean
  isRecord?: boolean
  language?: string
  pageCount?: number
  metadata?: Record<string, unknown>
  linkToCaseId?: string
}

/** CreateDocument response: `reused` when the content already had a live document. */
export interface CreateDocumentResult {
  document: GoDocument
  reused: boolean
}

export interface AddDocumentVersionRequest {
  contentBlobId?: string
  isFinal?: boolean
  isRecord?: boolean
  pageCount?: number
  metadata?: Record<string, unknown>
  reason?: string
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
  /** The registered content, passed to createDocument / addDocumentVersion. */
  contentBlobId: string
  /** True when identical content was already known (deduplicated). */
  reused: boolean
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

// ---- case -------------------------------------------------------------------

export type CaseStatus
  = | 'CASE_STATUS_UNSPECIFIED'
    | 'CASE_STATUS_OPEN'
    | 'CASE_STATUS_IN_PROGRESS'
    | 'CASE_STATUS_SUSPENDED'
    | 'CASE_STATUS_CLOSED'

export interface CaseType {
  id: string
  code: string
  label: string
  description?: string
  /** When set, CreateCase allocates the business reference in this namespace. */
  businessRefNamespace?: string
  isActive?: boolean
}

export interface GoCase {
  /** Canonical identity, including businessRef / businessRefNamespace. */
  subjectRef?: SubjectRef
  caseType?: CaseType
  title: string
  description?: string
  status?: CaseStatus
  openedAt?: string
  closedAt?: string
  closedBy?: string
  closureReason?: string
  metadata?: Record<string, unknown>
  createdAt?: string
  createdBy?: string
  updatedAt?: string
  recordMetadata?: RecordMetadata
}

export interface CreateCaseRequest {
  caseTypeCode: string
  title: string
  description?: string
  metadata?: Record<string, unknown>
}

export interface UpdateCaseRequest {
  title: string
  description?: string
  metadata?: Record<string, unknown>
  reason?: string
}

export interface GetCaseResponse {
  case?: GoCase
  /** Outgoing then incoming relationships. */
  relationships?: SubjectRelationship[]
  recentAudit?: AuditEvent[]
}

export interface SearchCasesParams {
  query?: string
  caseTypeCode?: string
  status?: CaseStatus
  includeDeleted?: boolean
  pageSize?: number
  pageToken?: string
}

export interface SearchCasesResponse {
  cases?: GoCase[]
  nextPageToken?: string
  totalSize?: number
}

// --- Internal users (CoreService, GLD-025) ------------------------------------

/** An internal, authenticated user (employee); governance/audit ids refer to User.id. */
export interface User {
  id: string
  subjectId?: string
  displayName?: string
  email?: string
  isAdmin?: boolean
  firstSeenAt?: string
  lastSeenAt?: string
}

export interface GetCurrentUserResponse {
  user?: User
  scopes?: string[]
}

export interface BatchGetUsersResponse {
  users?: User[]
}

// --- Reference data administration (GLD-040) ----------------------------------

/** A catalogue administered through the API. */
export type ReferenceCatalogue = 'case_type' | 'relationship_type' | 'organization_category' | 'document_type'

/** One entry of the append-only reference change log. */
export interface ReferenceChange {
  id: string
  catalogue: ReferenceCatalogue
  code: string
  eventType: 'REFERENCE_CREATED' | 'REFERENCE_UPDATED'
  actorUserId?: string
  occurredAt?: string
  beforeState?: Record<string, unknown>
  afterState?: Record<string, unknown>
  reason?: string
}

export interface ListReferenceChangesResponse {
  changes?: ReferenceChange[]
  nextPageToken?: string
  totalSize?: number
}
