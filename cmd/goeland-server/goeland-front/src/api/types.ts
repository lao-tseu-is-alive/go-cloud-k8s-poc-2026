/**
 * TypeScript projections of the goeland.v1 proto messages as serialized by the
 * Vanguard REST transcoder (proto3 JSON):
 *   - field names are camelCase,
 *   - int64 fields are serialized as strings (e.g. fileSizeBytes: "38"),
 *   - enums are their string names (e.g. "DOCUMENT_STATUS_DRAFT"),
 *   - Timestamps are RFC3339 strings,
 *   - google.protobuf.Struct (metadata) is a plain JSON object.
 *
 * These are hand-maintained (no runtime codegen) and only cover the fields the
 * UI actually consumes; ./typesContract.ts checks them at compile time against
 * the proto3-JSON types generated into ./gen (GLD-072).
 */

/** A JSON value, as google.protobuf.Value is serialized. */
export type JsonValue = string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue }

/** A google.protobuf.Struct (metadata, audit states): a plain JSON object. */
export type JsonObject = { [key: string]: JsonValue }

export type SubjectKind
  = | 'SUBJECT_KIND_UNSPECIFIED'
    | 'SUBJECT_KIND_CASE'
    | 'SUBJECT_KIND_DOCUMENT'
    | 'SUBJECT_KIND_THING'
    | 'SUBJECT_KIND_ACTOR'
    | 'SUBJECT_KIND_USER'
    | 'SUBJECT_KIND_ORG_UNIT'
    | 'SUBJECT_KIND_GROUP'

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
  metadata?: JsonObject
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
  metadata?: JsonObject
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
  /** Creation day bounds, YYYY-MM-DD in Europe/Zurich, and creator (GLD-074). */
  createdFrom?: string
  createdTo?: string
  createdBy?: string
  pageSize?: number
  pageToken?: string
  /** order_by: "<field>" or "<field> desc" (GLD-055). */
  orderBy?: string
}

export interface SearchActorsResponse {
  actors?: GoActor[]
  nextPageToken?: string
  totalSize?: number
  /** totalSize is a lower bound: counting stops at 10 000 matches (GLD-053). */
  totalSizeCapped?: boolean
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
  /** The role one end plays for the other, singular ("Client"); empty: shown by its labels (GLD-063). */
  roleLabel?: string
  /** The plural of roleLabel when not regular; empty: derived (utils/roleHeadings.ts). */
  roleLabelPlural?: string
  /** The end that plays the role. */
  roleEnd?: RoleEnd
  /** How the role joins the other kind ("de", "par", "sur", ...). */
  rolePreposition?: string
}

/** Which end of a relationship type plays its role (GLD-063). */
export type RoleEnd = 'ROLE_END_UNSPECIFIED' | 'ROLE_END_TARGET' | 'ROLE_END_SOURCE'

/** What a relationship list shows of the other end (GLD-063); set with includeDetails. */
export interface RelatedSubjectDetail {
  typeCode?: string
  typeLabel?: string
  /** OPEN, IN_PROGRESS, SUSPENDED or CLOSED for a case. */
  caseStatus?: string
  /** A case's opening date or a document's official date (YYYY-MM-DD). */
  date?: string
}

/** The edges of one relationship type in one direction, counted (GLD-063). */
export interface RelationshipGroup {
  relationshipType?: RelationshipType
  /** True when the subject is the source of the edges. */
  outgoing?: boolean
  otherKind?: SubjectKind
  count?: number
  /** Other ends that are cases not closed. */
  openCount?: number
  /** The split by case type or document type. */
  families?: RelationshipFamily[]
}

export interface RelationshipFamily {
  code: string
  label?: string
  count?: number
  openCount?: number
}

export interface SummarizeRelationshipsResponse {
  groups?: RelationshipGroup[]
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
  /** The other end, described (GLD-063); set by a list asked with includeDetails. */
  otherDetail?: RelatedSubjectDetail
}

export interface AuditEvent {
  id?: string
  subjectId?: string
  eventType?: string
  actorUserId?: string
  occurredAt?: string
  beforeState?: JsonObject
  afterState?: JsonObject
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
  /** Official date bounds, YYYY-MM-DD (GLD-074). */
  officialFrom?: string
  officialTo?: string
  /** Creation day bounds, YYYY-MM-DD in Europe/Zurich, and creator (GLD-074). */
  createdFrom?: string
  createdTo?: string
  createdBy?: string
  pageSize?: number
  pageToken?: string
  /** order_by: "<field>" or "<field> desc" (GLD-055). */
  orderBy?: string
}

export interface SearchDocumentsResponse {
  documents?: GoDocument[]
  nextPageToken?: string
  totalSize?: number
  /** totalSize is a lower bound: counting stops at 10 000 matches (GLD-053). */
  totalSizeCapped?: boolean
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
  metadata?: JsonObject
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
  metadata?: JsonObject
  reason?: string
}

export interface UpdateDocumentMetadataRequest {
  title?: string
  description?: string
  officialDate?: string
  language?: string
  metadata?: JsonObject
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
  /** Minimum confidentiality of new cases of this type (GLD-050). */
  defaultConfidentialityLevel?: number
  /** Grants copied once onto every new case of this type (GLD-050). */
  defaultGrants?: CaseTypeDefaultGrant[]
  /** Retention policy given to its cases when closed (GLD-032); absent when none. */
  defaultRetentionPolicyCode?: string
}

/** One line of a case type's default grants; granteeId is empty for the creator's units. */
export interface CaseTypeDefaultGrant {
  granteeKind: GranteeKind
  granteeId?: string
  granteeLabel?: string
  level: Permission
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
  metadata?: JsonObject
  createdAt?: string
  createdBy?: string
  updatedAt?: string
  recordMetadata?: RecordMetadata
}

export interface CreateCaseRequest {
  caseTypeCode: string
  title: string
  description?: string
  metadata?: JsonObject
}

export interface UpdateCaseRequest {
  title: string
  description?: string
  metadata?: JsonObject
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
  /** Opening day bounds, YYYY-MM-DD in Europe/Zurich (GLD-074). */
  openedFrom?: string
  openedTo?: string
  /** Creation day bounds, YYYY-MM-DD in Europe/Zurich, and creator (GLD-074). */
  createdFrom?: string
  createdTo?: string
  createdBy?: string
  pageSize?: number
  pageToken?: string
  /** order_by: "<field>" or "<field> desc" (GLD-055). */
  orderBy?: string
}

/** A page of the relationships of a subject, in one direction or both. */
export interface ListRelationshipsResponse {
  relationships?: SubjectRelationship[]
  nextPageToken?: string
  totalSize?: number
  totalSizeCapped?: boolean
}

export interface SearchCasesResponse {
  cases?: GoCase[]
  nextPageToken?: string
  totalSize?: number
  /** totalSize is a lower bound: counting stops at 10 000 matches (GLD-053). */
  totalSizeCapped?: boolean
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
  /** Codes of the application roles currently held (GLD-047), sorted. */
  roles?: string[]
}

/** An application role of the catalogue (ADMIN is the role behind goeland:admin). */
export interface AppRole {
  code: string
  label?: string
  description?: string
  isActive?: boolean
}

/** One assignment of a role to a user; a revoked one is kept as history. */
export interface UserRole {
  id: string
  userId: string
  roleCode: string
  grantedAt?: string
  grantedBy?: string
  grantReason?: string
  revokedAt?: string
  revokedBy?: string
  revokeReason?: string
}

export interface ListAppRolesResponse {
  roles?: AppRole[]
}

export interface ListRoleHoldersResponse {
  users?: User[]
}

export interface ListUserRolesResponse {
  roles?: UserRole[]
}

export interface UserRoleChangeResponse {
  role?: UserRole
  auditEvent?: AuditEvent
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
export type ReferenceCatalogue = 'case_type' | 'relationship_type' | 'organization_category' | 'document_type' | 'thing_type' | 'org_unit_type' | 'task_type' | 'retention_policy'

/** One entry of the append-only reference change log. */
export interface ReferenceChange {
  id: string
  catalogue: ReferenceCatalogue
  code: string
  eventType: 'REFERENCE_CREATED' | 'REFERENCE_UPDATED'
  actorUserId?: string
  occurredAt?: string
  beforeState?: JsonObject
  afterState?: JsonObject
  reason?: string
}

export interface ListReferenceChangesResponse {
  changes?: ReferenceChange[]
  nextPageToken?: string
  totalSize?: number
}

// --- Things (ThingService, GLD-016) -------------------------------------------

export type ThingSpecialization
  = | 'THING_SPECIALIZATION_UNSPECIFIED'
    | 'THING_SPECIALIZATION_PARCEL'
    | 'THING_SPECIALIZATION_BUILDING'

/** RegBL / GWR building status. */
export type BuildingStatus
  = | 'BUILDING_STATUS_UNSPECIFIED'
    | 'BUILDING_STATUS_PLANNED'
    | 'BUILDING_STATUS_AUTHORIZED'
    | 'BUILDING_STATUS_UNDER_CONSTRUCTION'
    | 'BUILDING_STATUS_EXISTING'
    | 'BUILDING_STATUS_UNUSABLE'
    | 'BUILDING_STATUS_DEMOLISHED'
    | 'BUILDING_STATUS_NOT_REALIZED'

export interface ThingType {
  id: string
  code: string
  label: string
  description?: string
  /** Absent means generic (proto3 JSON omits the zero value). */
  specialization?: ThingSpecialization
  isActive?: boolean
}

export interface ParcelDetails {
  communeOfs: number
  parcelNumber: string
  egrid?: string
  surfaceM2?: number
}

export interface BuildingDetails {
  egid?: number
  ecaNumber?: string
  constructionYear?: number
  buildingStatus?: BuildingStatus
}

export interface GoThing {
  subjectRef?: SubjectRef
  thingType?: ThingType
  name: string
  description?: string
  externalRef?: string
  /** GeoJSON geometry in EPSG:2056 (LV95); absent when not georeferenced. */
  geometryGeojson?: string
  areaM2?: number
  anchorE?: number
  anchorN?: number
  parcel?: ParcelDetails
  building?: BuildingDetails
  metadata?: JsonObject
  createdAt?: string
  createdBy?: string
  updatedAt?: string
  recordMetadata?: RecordMetadata
}

export interface CreateThingRequest {
  thingTypeCode: string
  name?: string
  description?: string
  externalRef?: string
  geometryGeojson?: string
  parcel?: ParcelDetails
  building?: BuildingDetails
}

export interface UpdateThingRequest {
  name: string
  description?: string
  externalRef?: string
  geometryGeojson?: string
  parcel?: ParcelDetails
  building?: BuildingDetails
  reason?: string
}

export interface GetThingResponse {
  thing?: GoThing
  relationships?: SubjectRelationship[]
  recentAudit?: AuditEvent[]
}

export interface SearchThingsParams {
  query?: string
  thingTypeCode?: string
  bbox?: string
  includeDeleted?: boolean
  /** Creation day bounds, YYYY-MM-DD in Europe/Zurich, and creator (GLD-074). */
  createdFrom?: string
  createdTo?: string
  createdBy?: string
  pageSize?: number
  pageToken?: string
  /** order_by: "<field>" or "<field> desc" (GLD-055). */
  orderBy?: string
}

export interface SearchThingsResponse {
  things?: GoThing[]
  nextPageToken?: string
  totalSize?: number
  /** totalSize is a lower bound: counting stops at 10 000 matches (GLD-053). */
  totalSizeCapped?: boolean
}

// ---------------------------------------------------------------------------
// Case timeline (TimelineService, timeline.proto)
// ---------------------------------------------------------------------------

export type TimelineEntryType
  = | 'TIMELINE_ENTRY_TYPE_UNSPECIFIED'
    | 'TIMELINE_ENTRY_TYPE_COMMENT'
    | 'TIMELINE_ENTRY_TYPE_OPINION'
    | 'TIMELINE_ENTRY_TYPE_DECISION'
    | 'TIMELINE_ENTRY_TYPE_REQUEST'
    | 'TIMELINE_ENTRY_TYPE_RESPONSE'
    | 'TIMELINE_ENTRY_TYPE_VALIDATION'
    | 'TIMELINE_ENTRY_TYPE_SYSTEM'
    | 'TIMELINE_ENTRY_TYPE_AI_PROPOSAL'

export type TimelineEntryStatus
  = | 'TIMELINE_ENTRY_STATUS_UNSPECIFIED'
    | 'TIMELINE_ENTRY_STATUS_DRAFT'
    | 'TIMELINE_ENTRY_STATUS_VALIDATED'
    | 'TIMELINE_ENTRY_STATUS_LOCKED'
    | 'TIMELINE_ENTRY_STATUS_WITHDRAWN'

export type TimelineVisibility
  = | 'TIMELINE_VISIBILITY_UNSPECIFIED'
    | 'TIMELINE_VISIBILITY_CASE_PARTICIPANTS'
    | 'TIMELINE_VISIBILITY_INTERNAL'
    | 'TIMELINE_VISIBILITY_RESTRICTED'

/** A document cited by an entry; the version is pinned once the entry leaves the draft state. */
export interface TimelineDocumentLink {
  id: string
  documentId: string
  documentLabel?: string
  /** Empty while the entry is a draft. */
  documentVersionId?: string
  /** 0 (absent in JSON) when no version is pinned. */
  documentVersionNo?: number
  createdAt?: string
  createdBy?: string
}

export interface TimelineEntry {
  id: string
  caseId: string
  entryType?: TimelineEntryType
  status?: TimelineEntryStatus
  title?: string
  body: string
  visibility?: TimelineVisibility
  /** Business date of the event; the timeline is ordered on it. */
  occurredAt?: string
  correctsEntryId?: string
  correctedByEntryId?: string
  documents?: TimelineDocumentLink[]
  createdAt?: string
  createdBy?: string
  updatedAt?: string
  updatedBy?: string
  validatedAt?: string
  validatedBy?: string
  lockedAt?: string
  lockedBy?: string
  withdrawnAt?: string
  withdrawnBy?: string
  withdrawalReason?: string
  /** For a SYSTEM entry: { event, from, to, reason } describing the fact. */
  metadata?: JsonObject
}

export interface ListTimelineParams {
  entryTypes?: TimelineEntryType[]
  includeWithdrawn?: boolean
  pageSize?: number
  pageToken?: string
}

export interface ListTimelineResponse {
  entries?: TimelineEntry[]
  nextPageToken?: string
  totalSize?: number
  /** Drafts of the case whatever the filters (0 is absent in JSON). */
  draftCount?: number
}

/** Operator-editable content shared by create and update. */
export interface TimelineEntryContent {
  entryType: TimelineEntryType
  title?: string
  body: string
  visibility?: TimelineVisibility
  occurredAt?: string
}

export interface CreateTimelineEntryRequest extends TimelineEntryContent {
  correctsEntryId?: string
  documentIds?: string[]
}

export interface UpdateTimelineEntryRequest extends TimelineEntryContent {
  reason?: string
}

// ---------------------------------------------------------------------------
// Organizational units (OrgUnitService, orgunit.proto)
// ---------------------------------------------------------------------------

export interface OrgUnitType {
  id?: string
  code: string
  label: string
  description?: string
  sortOrder?: number
  isActive?: boolean
}

/** Light unit used to draw the tree and paths. */
export interface OrgUnitNode {
  id: string
  abbreviation?: string
  label: string
  orgUnitTypeCode?: string
  /** Empty (absent in JSON) for a root. */
  parentId?: string
  dissolved?: boolean
}

export interface OrgUnit {
  subjectRef?: SubjectRef
  orgUnitType?: OrgUnitType
  /** Sigle, often shared with the parent unit (not unique). */
  abbreviation?: string
  label: string
  description?: string
  email?: string
  parentId?: string
  dissolvedAt?: string
  dissolvedBy?: string
  dissolutionReason?: string
  createdAt?: string
  createdBy?: string
  updatedAt?: string
  recordMetadata?: RecordMetadata
  /** Id in a source system (e.g. goeland:1234); immutable. */
  externalRef?: string
}

export interface GetOrgUnitResponse {
  orgUnit?: OrgUnit
  /** From the root down to the parent. */
  ancestors?: OrgUnitNode[]
  children?: OrgUnitNode[]
  relationships?: SubjectRelationship[]
  recentAudit?: AuditEvent[]
}

/** Editable fields shared by create and update (full replacement on update). */
export interface OrgUnitInput {
  orgUnitTypeCode: string
  abbreviation?: string
  label: string
  description?: string
  email?: string
  parentId?: string
  reason?: string
}

export interface CreateOrgUnitRequest extends OrgUnitInput {
  externalRef?: string
}

export interface SearchOrgUnitsParams {
  query?: string
  includeDissolved?: boolean
  pageSize?: number
  pageToken?: string
}

export interface SearchOrgUnitsResponse {
  units?: OrgUnit[]
  nextPageToken?: string
  totalSize?: number
}

// ---------------------------------------------------------------------------
// Case tasks (TaskService, task.proto)
// ---------------------------------------------------------------------------

export type TaskStatus
  = | 'TASK_STATUS_UNSPECIFIED'
    | 'TASK_STATUS_OPEN'
    | 'TASK_STATUS_IN_PROGRESS'
    | 'TASK_STATUS_DONE'
    | 'TASK_STATUS_CANCELLED'

export type TaskOrigin
  = | 'TASK_ORIGIN_UNSPECIFIED'
    | 'TASK_ORIGIN_MANUAL'
    | 'TASK_ORIGIN_CIRCULATION'
    | 'TASK_ORIGIN_WORKFLOW'
    | 'TASK_ORIGIN_AI'

export interface TaskType {
  id?: string
  code: string
  label: string
  description?: string
  isActive?: boolean
}

export interface TaskAssignment {
  id: string
  assigneeUserId?: string
  assigneeOrgUnitId?: string
  assigneeLabel?: string
  assignedAt?: string
  assignedBy?: string
  reason?: string
  /** Absent for the current assignment. */
  endedAt?: string
}

export interface Task {
  id: string
  caseId: string
  /** Business reference and title of the case. */
  caseLabel?: string
  taskType?: TaskType
  title: string
  description?: string
  status?: TaskStatus
  origin?: TaskOrigin
  originRef?: string
  assigneeUserId?: string
  assigneeOrgUnitId?: string
  assigneeLabel?: string
  dueAt?: string
  /** A pending task past its deadline (absent in JSON when false). */
  overdue?: boolean
  createdAt?: string
  createdBy?: string
  updatedAt?: string
  updatedBy?: string
  startedAt?: string
  startedBy?: string
  completedAt?: string
  completedBy?: string
  completionNote?: string
  cancelledAt?: string
  cancelledBy?: string
  cancellationReason?: string
  /** Assignment history, oldest first (GetTask only). */
  assignments?: TaskAssignment[]
}

export interface ListMyTasksResponse {
  tasks?: Task[]
  nextPageToken?: string
  totalSize?: number
}

export interface ListCaseTasksResponse extends ListMyTasksResponse {
  /** Pending tasks of the case whatever the filters. */
  openCount?: number
}

/** Editable content of a task; update replaces every field (absent dueAt removes it). */
export interface TaskContent {
  taskTypeCode: string
  title: string
  description?: string
  dueAt?: string
}

export interface CreateTaskRequest extends TaskContent {
  assigneeUserId?: string
  assigneeOrgUnitId?: string
}

export interface SearchUsersResponse {
  users?: User[]
}

// --- Global search (CoreService.SearchSubjects, GLD-064) ---------------------

export interface SearchSubjectsParams {
  /** Matched from the start of every word of the label and reference, and exactly against the reference. */
  query: string
  /** One result group per kind, in this order; empty means case, actor, document, thing. */
  kinds?: SubjectKind[]
  /** Subjects per kind; 0 or absent means 5, at most 25. */
  limitPerKind?: number
}

export interface SubjectSearchGroup {
  kind: SubjectKind
  subjects?: SubjectRef[]
  /** Readable matches, at most 500. */
  totalSize?: number
  /** True beyond 500 matches: totalSize is a lower bound. */
  totalSizeCapped?: boolean
}

export interface SearchSubjectsResponse {
  groups?: SubjectSearchGroup[]
}

// ---------------------------------------------------------------------------
// Case circulations (CirculationService, circulation.proto)
// ---------------------------------------------------------------------------

export type CirculationStatus
  = | 'CIRCULATION_STATUS_UNSPECIFIED'
    | 'CIRCULATION_STATUS_OPEN'
    | 'CIRCULATION_STATUS_COMPLETED'
    | 'CIRCULATION_STATUS_CANCELLED'

export type CirculationResponse
  = | 'CIRCULATION_RESPONSE_UNSPECIFIED'
    | 'CIRCULATION_RESPONSE_FAVORABLE'
    | 'CIRCULATION_RESPONSE_UNFAVORABLE'
    | 'CIRCULATION_RESPONSE_COMMENT'
    | 'CIRCULATION_RESPONSE_NOT_CONCERNED'
    | 'CIRCULATION_RESPONSE_NEED_MORE_INFO'

export interface CirculationRecipient {
  id: string
  step?: number
  assigneeUserId?: string
  assigneeOrgUnitId?: string
  assigneeLabel?: string
  /** Empty until the recipient's step opens. */
  taskId?: string
  taskStatus?: TaskStatus
  /** Step open and not answered yet (absent in JSON when false). */
  awaiting?: boolean
  response?: CirculationResponse
  responseText?: string
  respondedAt?: string
  respondedBy?: string
  responseEntryId?: string
}

export interface Circulation {
  id: string
  caseId: string
  title: string
  message?: string
  dueAt?: string
  overdue?: boolean
  status?: CirculationStatus
  currentStep?: number
  stepCount?: number
  createdAt?: string
  createdBy?: string
  completedAt?: string
  cancelledAt?: string
  cancelledBy?: string
  cancellationReason?: string
  recipients?: CirculationRecipient[]
}

export interface CirculationRecipientInput {
  step?: number
  assigneeUserId?: string
  assigneeOrgUnitId?: string
}

export interface CreateCirculationRequest {
  title: string
  message?: string
  dueAt?: string
  recipients: CirculationRecipientInput[]
}

// ---- Access: grants and security groups (GLD-048) ------------------------------------

/** CREATOR_UNITS only appears in a case type's default grants (the creator's units, resolved at creation). */
export type GranteeKind = 'GRANTEE_KIND_UNSPECIFIED' | 'GRANTEE_KIND_USER' | 'GRANTEE_KIND_GROUP' | 'GRANTEE_KIND_ORG_UNIT' | 'GRANTEE_KIND_CREATOR_UNITS'

export type AccessSource
  = | 'ACCESS_SOURCE_UNSPECIFIED'
    | 'ACCESS_SOURCE_PERSONAL'
    | 'ACCESS_SOURCE_GROUP'
    | 'ACCESS_SOURCE_ORG_UNIT'
    | 'ACCESS_SOURCE_ROLE'
    | 'ACCESS_SOURCE_BASELINE'

/** The caller's effective level on a subject and where it comes from. */
export interface Access {
  subjectId: string
  kind?: SubjectKind
  confidential?: boolean
  level?: Permission
  source?: AccessSource
}

/** What a sensitive access did (GLD-033). */
export type AccessAuditAction = 'READ_SENSITIVE' | 'DOWNLOAD' | 'EXPORT'

/** Whether a sensitive access was allowed. */
export type AccessAuditOutcome = 'GRANTED' | 'DENIED'

/** One row of the sensitive read audit: a read or download of a confidential subject. */
export interface AccessAuditEvent {
  id: string
  occurredAt?: string
  userId?: string
  subjectId: string
  subjectKind?: SubjectKind
  action?: AccessAuditAction
  outcome?: AccessAuditOutcome
  accessLevel?: Permission
  accessSource?: AccessSource
  confidentialityLevel?: number
  documentVersionId?: string
  requestId?: string
  businessRef?: string
  /** Empty when the caller may not read the subject: the log never discloses a label. */
  subjectLabel?: string
}

export interface ListAccessAuditEventsResponse {
  events?: AccessAuditEvent[]
  nextPageToken?: string
  totalSize?: number
  totalSizeCapped?: boolean
}

/** One grant on a subject; a revoked one is kept as history. */
export interface AccessGrant {
  id: string
  subjectId: string
  granteeKind?: GranteeKind
  granteeId?: string
  granteeLabel?: string
  level?: Permission
  grantedAt?: string
  grantedBy?: string
  grantReason?: string
  revokedAt?: string
  revokedBy?: string
  revokeReason?: string
}

/** A security group: a named set of internal users given grants like a unit. */
export interface SecurityGroup {
  subjectRef?: SubjectRef
  name: string
  description?: string
  archivedAt?: string
  memberCount?: number
}

/** A current member of a group. */
export interface GroupMember {
  user?: User
  relationshipId?: string
  since?: string
  addedBy?: string
}

export interface GetMyAccessResponse {
  access?: Access
}

export interface ListGrantsResponse {
  grants?: AccessGrant[]
}

export interface GrantChangeResponse {
  grant?: AccessGrant
  auditEvent?: AuditEvent
}

export interface ListGroupsResponse {
  groups?: SecurityGroup[]
}

export interface GroupResponse {
  group?: SecurityGroup
  members?: GroupMember[]
}

/** Answer of a group creation, edit or archiving. */
export interface GroupChangeResponse {
  group?: SecurityGroup
  auditEvent?: AuditEvent
}

/** Answer of a membership change (no member after a removal). */
export interface GroupMemberChangeResponse {
  member?: GroupMember
  auditEvent?: AuditEvent
}

// --- Case export (GLD-029) ----------------------------------------------------

/** State of a case export job. */
export type CaseExportStatus
  = | 'CASE_EXPORT_STATUS_UNSPECIFIED'
    | 'CASE_EXPORT_STATUS_QUEUED'
    | 'CASE_EXPORT_STATUS_RUNNING'
    | 'CASE_EXPORT_STATUS_DONE'
    | 'CASE_EXPORT_STATUS_FAILED'
    | 'CASE_EXPORT_STATUS_EXPIRED'

/** One export job of a case; only its requester downloads the archive. */
export interface CaseExport {
  id: string
  caseId: string
  status?: CaseExportStatus
  requestedBy?: string
  reason?: string
  requestedAt?: string
  startedAt?: string
  finishedAt?: string
  expiresAt?: string
  /** int64 serialized as a string. */
  sizeBytes?: string
  sha256?: string
  error?: string
}

export interface CaseExportResponse {
  export?: CaseExport
}

export interface ListCaseExportsResponse {
  exports?: CaseExport[]
}

// ---------------------------------------------------------------------------
// Transactional outbox (CoreService, GLD-028)
// ---------------------------------------------------------------------------

/** A change announced to the outbox consumers: one per audit event. */
export interface OutboxEvent {
  id: string
  subjectKind?: SubjectKind
  subjectId?: string
  eventType?: string
  auditEventId?: string
  occurredAt?: string
  attempts?: number
  nextAttemptAt?: string
  publishedAt?: string
  failedAt?: string
  lastError?: string
  failedConsumers?: string[]
}

/** A consumer of the outbox and the given-up events it refused. */
export interface OutboxConsumerStatus {
  name: string
  failed?: number
}

/** The state of the outbox (GetOutboxStatus). */
export interface OutboxStatus {
  pending?: number
  failed?: number
  published?: number
  oldestPendingAt?: string
  retentionDays?: number
  maxAttempts?: number
  consumers?: OutboxConsumerStatus[]
  recentFailed?: OutboxEvent[]
}

// ---------------------------------------------------------------------------
// Retention of closed cases (RetentionService, GLD-032)
// ---------------------------------------------------------------------------

/** What happens to a case when its retention expires. */
export type FinalDisposition
  = | 'FINAL_DISPOSITION_UNSPECIFIED'
    | 'FINAL_DISPOSITION_ARCHIVE'
    | 'FINAL_DISPOSITION_DESTROY'
    | 'FINAL_DISPOSITION_REVIEW'

/** An entry of the retention schedule. */
export interface RetentionPolicy {
  id: string
  code: string
  label: string
  description?: string
  legalBasis?: string
  /** Years kept after the closing (0 is absent in JSON). */
  durationYears?: number
  finalDisposition?: FinalDisposition
  isActive?: boolean
}

export interface ListRetentionPoliciesResponse {
  retentionPolicies?: RetentionPolicy[]
}

/** The retention of a closed case: a snapshot of its policy taken at closing. */
export interface SubjectRetention {
  id: string
  subjectId: string
  policyCode: string
  policyLabel?: string
  durationYears?: number
  finalDisposition?: FinalDisposition
  /** Closing day, YYYY-MM-DD (Europe/Zurich). */
  startsOn: string
  /** Day the retention ends: the case is due from that day. */
  untilOn: string
  createdAt?: string
  createdBy?: string
  endedAt?: string
  endedBy?: string
  endReason?: string
  /** Executed outcome: transferred and frozen, or eliminated. */
  dispositionStatus?: DispositionStatus
  disposedAt?: string
  bordereauId?: string
}

export type DispositionStatus = 'DISPOSITION_STATUS_UNSPECIFIED' | 'DISPOSITION_STATUS_ARCHIVED' | 'DISPOSITION_STATUS_DISPOSED'

/** A legal hold: no disposal while it is active. */
export interface RetentionHold {
  id: string
  subjectId: string
  reason: string
  placedAt?: string
  placedBy?: string
  releasedAt?: string
  releasedBy?: string
  releaseReason?: string
}

export interface GetCaseRetentionResponse {
  current?: SubjectRetention
  hold?: RetentionHold
  retentions?: SubjectRetention[]
  holds?: RetentionHold[]
  defaultPolicy?: RetentionPolicy
}

/** Answer of a hold placed or released. */
export interface RetentionHoldResponse {
  hold?: RetentionHold
  auditEvent?: AuditEvent
}

/** A due retention with its case; caseLabel is empty for a case the caller may not read. */
export interface DueRetention {
  retention: SubjectRetention
  caseLabel?: string
  businessRef?: string
  caseTypeCode?: string
  onHold?: boolean
}

export interface ListDueRetentionsResponse {
  retentions?: DueRetention[]
  nextPageToken?: string
  totalSize?: number
}

export interface ApplyCaseTypeRetentionResponse {
  applied?: number
  /** Closed cases of the type still without retention: call again while > 0. */
  remaining?: number
}

// --- Disposition bordereaux (GLD-032 step B) --------------------------------

export type BordereauStatus = 'BORDEREAU_STATUS_UNSPECIFIED' | 'BORDEREAU_STATUS_OPEN' | 'BORDEREAU_STATUS_EXECUTED' | 'BORDEREAU_STATUS_CANCELLED'

export type DispositionDecision = 'DISPOSITION_DECISION_UNSPECIFIED' | 'DISPOSITION_DECISION_PENDING' | 'DISPOSITION_DECISION_APPROVED' | 'DISPOSITION_DECISION_REJECTED'

/** One case of a bordereau; labels are empty when the caller may not read the subject. */
export interface DispositionItem {
  id: string
  bordereauId: string
  caseId: string
  caseLabel?: string
  businessRef?: string
  untilOn?: string
  policyCode?: string
  /** The owning unit, or the case itself without one. */
  approverSubjectId: string
  approverLabel?: string
  decision?: DispositionDecision
  decidedAt?: string
  decidedBy?: string
  decisionReason?: string
  /** The transfer package of an executed ARCHIVE item. */
  exportId?: string
  exportStatus?: string
  exportSha256?: string
}

export interface DispositionBordereau {
  id: string
  finalDisposition?: FinalDisposition
  status?: BordereauStatus
  reason: string
  preparedBy?: string
  preparedAt?: string
  closedAt?: string
  closedBy?: string
  closeReason?: string
  proofSha256?: string
  items?: DispositionItem[]
  itemCount?: number
  pendingCount?: number
  approvedCount?: number
}

/** Answer of a bordereau created, read, executed or cancelled. */
export interface DispositionBordereauResponse {
  bordereau?: DispositionBordereau
}

export interface ListDispositionBordereauxResponse {
  bordereaux?: DispositionBordereau[]
  nextPageToken?: string
  totalSize?: number
}

export interface ListMyDispositionApprovalsResponse {
  items?: DispositionItem[]
  /** The final disposition of each item's bordereau, by item id. */
  dispositions?: Record<string, FinalDisposition>
}

export interface DecideDispositionItemResponse {
  item?: DispositionItem
}
