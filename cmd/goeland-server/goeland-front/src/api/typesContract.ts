/**
 * Compile-time contract between the hand-written REST types (./types) and the
 * proto3-JSON types generated from the protos into ./gen (GLD-072). Each line
 * requires a hand-written type to name only fields of its message, with
 * compatible types, and an enum to list exactly the proto's values, so a field
 * renamed or retyped or an enum value added in a proto fails `bun run
 * type-check` (and `make check`) until the SPA follows. Type-only: nothing here
 * reaches the bundle. On a `notAssignable` failure, assigning a value of the
 * hand-written type to the generated one shows the offending field.
 *
 * Every export of ./types is either checked below or listed in
 * UNCHECKED_TYPES with the reason (src/api/__tests__/typesContract.test.ts).
 */

import type * as accessPb from './gen/goeland/v1/access_pb'
import type * as actorPb from './gen/goeland/v1/actor_pb'
import type * as casePb from './gen/goeland/v1/case_pb'
import type * as circulationPb from './gen/goeland/v1/circulation_pb'
import type * as corePb from './gen/goeland/v1/core_pb'
import type * as documentPb from './gen/goeland/v1/document_pb'
import type * as exportPb from './gen/goeland/v1/export_pb'
import type * as orgunitPb from './gen/goeland/v1/orgunit_pb'
import type * as retentionPb from './gen/goeland/v1/retention_pb'
import type * as taskPb from './gen/goeland/v1/task_pb'
import type * as thingPb from './gen/goeland/v1/thing_pb'
import type * as timelinePb from './gen/goeland/v1/timeline_pb'
import type * as T from './types'

/** 'ok' when every field of H is a field of G with an assignable type. */
type Conforms<H, G> = [Exclude<keyof H, keyof G>] extends [never]
  ? ([H] extends [G] ? 'ok' : { notAssignable: H })
  : { unknownFields: Exclude<keyof H, keyof G> }

/** 'ok' when two enums list the same values. */
type SameEnum<H, G> = [Exclude<H, G>] extends [never]
  ? ([Exclude<G, H>] extends [never] ? 'ok' : { missingInSpa: Exclude<G, H> })
  : { unknownInProto: Exclude<H, G> }

type Ok<T extends 'ok'> = T

/** The hand-written types that have no proto message, and why. */
export const UNCHECKED_TYPES: Readonly<Record<string, string>> = {
  JsonValue: 'google.protobuf.Value, checked through the messages that hold it',
  JsonObject: 'google.protobuf.Struct, checked through the messages that hold it',
  AccessAuditAction: 'values of the string field AccessAuditEvent.action',
  AccessAuditOutcome: 'values of the string field AccessAuditEvent.outcome',
  ReferenceCatalogue: 'values of the string field ReferenceChange.catalogue',
  TaskContent: 'fragment checked through CreateTaskRequest and UpdateTaskRequest',
  TimelineEntryContent: 'fragment checked through the timeline entry requests',
  FrontendConfig: 'out-of-proto GET /config',
  TokenResponse: 'answer of the authentication server',
  TokenUser: 'answer of the authentication server',
  UploadResult: 'out-of-proto POST /api/documents/upload',
}

export type TypesContract = [
  Ok<SameEnum<T.SubjectKind, corePb.SubjectKindJson>>,
  Ok<SameEnum<T.Permission, corePb.PermissionJson>>,
  Ok<SameEnum<T.DocumentStatus, documentPb.DocumentStatusJson>>,
  Ok<Conforms<T.SubjectRef, corePb.SubjectRefJson>>,
  Ok<Conforms<T.RecordMetadata, corePb.RecordMetadataJson>>,
  Ok<Conforms<T.DocumentType, documentPb.DocumentTypeJson>>,
  Ok<Conforms<T.ContentBlob, documentPb.ContentBlobJson>>,
  Ok<Conforms<T.DocumentVersion, documentPb.DocumentVersionJson>>,
  Ok<SameEnum<T.ActorKind, actorPb.ActorKindJson>>,
  Ok<SameEnum<T.ContactType, actorPb.ContactTypeJson>>,
  Ok<Conforms<T.OrganizationCategory, actorPb.OrganizationCategoryJson>>,
  Ok<Conforms<T.ActorContact, actorPb.ActorContactJson>>,
  Ok<Conforms<T.OrganizationDetails, actorPb.OrganizationDetailsJson>>,
  Ok<SameEnum<T.Salutation, actorPb.SalutationJson>>,
  Ok<Conforms<T.PersonDetails, actorPb.PersonDetailsJson>>,
  Ok<SameEnum<T.AddressType, actorPb.AddressTypeJson>>,
  Ok<Conforms<T.ActorAddress, actorPb.ActorAddressJson>>,
  Ok<Conforms<T.SearchActorsResponse, actorPb.SearchActorsResponseJson>>,
  Ok<Conforms<T.GetActorResponse, actorPb.GetActorResponseJson>>,
  Ok<Conforms<T.CreateActorRequest, actorPb.CreateActorRequestJson>>,
  Ok<Conforms<T.UpdateActorRequest, actorPb.UpdateActorRequestJson>>,
  Ok<Conforms<T.RelationshipType, corePb.RelationshipTypeJson>>,
  Ok<SameEnum<T.RoleEnd, corePb.RoleEndJson>>,
  Ok<Conforms<T.RelatedSubjectDetail, corePb.RelatedSubjectDetailJson>>,
  Ok<Conforms<T.RelationshipGroup, corePb.RelationshipGroupJson>>,
  Ok<Conforms<T.RelationshipFamily, corePb.RelationshipFamilyJson>>,
  Ok<Conforms<T.SummarizeRelationshipsResponse, corePb.SummarizeRelationshipsResponseJson>>,
  Ok<Conforms<T.SubjectRelationship, corePb.SubjectRelationshipJson>>,
  Ok<Conforms<T.AuditEvent, corePb.AuditEventJson>>,
  Ok<Conforms<T.SearchDocumentsResponse, documentPb.SearchDocumentsResponseJson>>,
  Ok<Conforms<T.GetDocumentResponse, documentPb.GetDocumentResponseJson>>,
  Ok<Conforms<T.CreateDocumentRequest, documentPb.CreateDocumentRequestJson>>,
  Ok<Conforms<T.AddDocumentVersionRequest, documentPb.AddDocumentVersionRequestJson>>,
  Ok<Conforms<T.UpdateDocumentMetadataRequest, documentPb.UpdateDocumentMetadataRequestJson>>,
  Ok<SameEnum<T.CaseStatus, casePb.CaseStatusJson>>,
  Ok<Conforms<T.CaseType, casePb.CaseTypeJson>>,
  Ok<Conforms<T.CaseTypeDefaultGrant, casePb.CaseTypeDefaultGrantJson>>,
  Ok<Conforms<T.CreateCaseRequest, casePb.CreateCaseRequestJson>>,
  Ok<Conforms<T.UpdateCaseRequest, casePb.UpdateCaseRequestJson>>,
  Ok<Conforms<T.GetCaseResponse, casePb.GetCaseResponseJson>>,
  Ok<Conforms<T.ListRelationshipsResponse, corePb.ListRelationshipsResponseJson>>,
  Ok<Conforms<T.SearchCasesResponse, casePb.SearchCasesResponseJson>>,
  Ok<Conforms<T.User, corePb.UserJson>>,
  Ok<Conforms<T.AppRole, corePb.AppRoleJson>>,
  Ok<Conforms<T.UserRole, corePb.UserRoleJson>>,
  Ok<Conforms<T.ListAppRolesResponse, corePb.ListAppRolesResponseJson>>,
  Ok<Conforms<T.ListRoleHoldersResponse, corePb.ListRoleHoldersResponseJson>>,
  Ok<Conforms<T.ListUserRolesResponse, corePb.ListUserRolesResponseJson>>,
  Ok<Conforms<T.GetCurrentUserResponse, corePb.GetCurrentUserResponseJson>>,
  Ok<Conforms<T.BatchGetUsersResponse, corePb.BatchGetUsersResponseJson>>,
  Ok<Conforms<T.ReferenceChange, corePb.ReferenceChangeJson>>,
  Ok<Conforms<T.ListReferenceChangesResponse, corePb.ListReferenceChangesResponseJson>>,
  Ok<SameEnum<T.ThingSpecialization, thingPb.ThingSpecializationJson>>,
  Ok<SameEnum<T.BuildingStatus, thingPb.BuildingStatusJson>>,
  Ok<Conforms<T.ThingType, thingPb.ThingTypeJson>>,
  Ok<Conforms<T.ParcelDetails, thingPb.ParcelDetailsJson>>,
  Ok<Conforms<T.BuildingDetails, thingPb.BuildingDetailsJson>>,
  Ok<Conforms<T.CreateThingRequest, thingPb.CreateThingRequestJson>>,
  Ok<Conforms<T.UpdateThingRequest, thingPb.UpdateThingRequestJson>>,
  Ok<Conforms<T.GetThingResponse, thingPb.GetThingResponseJson>>,
  Ok<Conforms<T.SearchThingsResponse, thingPb.SearchThingsResponseJson>>,
  Ok<SameEnum<T.TimelineEntryType, timelinePb.TimelineEntryTypeJson>>,
  Ok<SameEnum<T.TimelineEntryStatus, timelinePb.TimelineEntryStatusJson>>,
  Ok<SameEnum<T.TimelineVisibility, timelinePb.TimelineVisibilityJson>>,
  Ok<Conforms<T.TimelineDocumentLink, timelinePb.TimelineDocumentLinkJson>>,
  Ok<Conforms<T.TimelineEntry, timelinePb.TimelineEntryJson>>,
  Ok<Conforms<T.CreateTimelineEntryRequest, timelinePb.CreateTimelineEntryRequestJson>>,
  Ok<Conforms<T.UpdateTimelineEntryRequest, timelinePb.UpdateTimelineEntryRequestJson>>,
  Ok<Conforms<T.OrgUnitType, orgunitPb.OrgUnitTypeJson>>,
  Ok<Conforms<T.OrgUnitNode, orgunitPb.OrgUnitNodeJson>>,
  Ok<Conforms<T.OrgUnit, orgunitPb.OrgUnitJson>>,
  Ok<Conforms<T.GetOrgUnitResponse, orgunitPb.GetOrgUnitResponseJson>>,
  Ok<Conforms<T.CreateOrgUnitRequest, orgunitPb.CreateOrgUnitRequestJson>>,
  Ok<Conforms<T.SearchOrgUnitsResponse, orgunitPb.SearchOrgUnitsResponseJson>>,
  Ok<SameEnum<T.TaskStatus, taskPb.TaskStatusJson>>,
  Ok<SameEnum<T.TaskOrigin, taskPb.TaskOriginJson>>,
  Ok<Conforms<T.TaskType, taskPb.TaskTypeJson>>,
  Ok<Conforms<T.TaskAssignment, taskPb.TaskAssignmentJson>>,
  Ok<Conforms<T.Task, taskPb.TaskJson>>,
  Ok<Conforms<T.ListMyTasksResponse, taskPb.ListMyTasksResponseJson>>,
  Ok<Conforms<T.ListCaseTasksResponse, taskPb.ListCaseTasksResponseJson>>,
  Ok<Conforms<T.CreateTaskRequest, taskPb.CreateTaskRequestJson>>,
  Ok<Conforms<T.SearchUsersResponse, corePb.SearchUsersResponseJson>>,
  Ok<Conforms<T.SubjectSearchGroup, corePb.SubjectSearchGroupJson>>,
  Ok<Conforms<T.SearchSubjectsResponse, corePb.SearchSubjectsResponseJson>>,
  Ok<SameEnum<T.CirculationStatus, circulationPb.CirculationStatusJson>>,
  Ok<SameEnum<T.CirculationResponse, circulationPb.CirculationResponseJson>>,
  Ok<Conforms<T.CirculationRecipient, circulationPb.CirculationRecipientJson>>,
  Ok<Conforms<T.Circulation, circulationPb.CirculationJson>>,
  Ok<Conforms<T.CirculationRecipientInput, circulationPb.CirculationRecipientInputJson>>,
  Ok<Conforms<T.CreateCirculationRequest, circulationPb.CreateCirculationRequestJson>>,
  Ok<SameEnum<T.GranteeKind, accessPb.GranteeKindJson>>,
  Ok<SameEnum<T.AccessSource, corePb.AccessSourceJson>>,
  Ok<Conforms<T.Access, accessPb.AccessJson>>,
  Ok<Conforms<T.AccessAuditEvent, corePb.AccessAuditEventJson>>,
  Ok<Conforms<T.ListAccessAuditEventsResponse, corePb.ListAccessAuditEventsResponseJson>>,
  Ok<Conforms<T.AccessGrant, accessPb.AccessGrantJson>>,
  Ok<Conforms<T.SecurityGroup, accessPb.SecurityGroupJson>>,
  Ok<Conforms<T.GroupMember, accessPb.GroupMemberJson>>,
  Ok<Conforms<T.GetMyAccessResponse, accessPb.GetMyAccessResponseJson>>,
  Ok<Conforms<T.ListGrantsResponse, accessPb.ListGrantsResponseJson>>,
  Ok<Conforms<T.ListGroupsResponse, accessPb.ListGroupsResponseJson>>,
  Ok<SameEnum<T.CaseExportStatus, exportPb.CaseExportStatusJson>>,
  Ok<Conforms<T.CaseExport, exportPb.CaseExportJson>>,
  Ok<Conforms<T.ListCaseExportsResponse, exportPb.ListCaseExportsResponseJson>>,
  Ok<Conforms<T.OutboxEvent, corePb.OutboxEventJson>>,
  Ok<Conforms<T.OutboxConsumerStatus, corePb.OutboxConsumerStatusJson>>,
  Ok<Conforms<T.CaseExportResponse, exportPb.RequestCaseExportResponseJson>>,
  Ok<Conforms<T.CaseExportResponse, exportPb.GetCaseExportResponseJson>>,
  Ok<Conforms<T.CreateDocumentResult, documentPb.CreateDocumentResponseJson>>,
  Ok<Conforms<T.GoActor, actorPb.ActorJson>>,
  Ok<Conforms<T.GoCase, casePb.CaseJson>>,
  Ok<Conforms<T.GoDocument, documentPb.DocumentJson>>,
  Ok<Conforms<T.GoThing, thingPb.ThingJson>>,
  Ok<Conforms<T.GrantChangeResponse, accessPb.SetGrantResponseJson>>,
  Ok<Conforms<T.GrantChangeResponse, accessPb.RevokeGrantResponseJson>>,
  Ok<Conforms<T.GroupResponse, accessPb.GetGroupResponseJson>>,
  Ok<Conforms<T.GroupChangeResponse, accessPb.CreateGroupResponseJson>>,
  Ok<Conforms<T.GroupChangeResponse, accessPb.UpdateGroupResponseJson>>,
  Ok<Conforms<T.GroupChangeResponse, accessPb.ArchiveGroupResponseJson>>,
  Ok<Conforms<T.GroupMemberChangeResponse, accessPb.AddGroupMemberResponseJson>>,
  Ok<Conforms<Pick<T.GroupMemberChangeResponse, 'auditEvent'>, accessPb.RemoveGroupMemberResponseJson>>,
  Ok<Conforms<T.ListTimelineParams, timelinePb.ListTimelineEntriesRequestJson>>,
  Ok<Conforms<T.ListTimelineResponse, timelinePb.ListTimelineEntriesResponseJson>>,
  Ok<Conforms<T.OrgUnitInput, orgunitPb.CreateOrgUnitRequestJson>>,
  Ok<Conforms<T.OrgUnitInput, orgunitPb.UpdateOrgUnitRequestJson>>,
  Ok<Conforms<T.OutboxStatus, corePb.GetOutboxStatusResponseJson>>,
  Ok<Conforms<T.SearchActorsParams, actorPb.SearchActorsRequestJson>>,
  Ok<Conforms<T.SearchCasesParams, casePb.SearchCasesRequestJson>>,
  Ok<Conforms<T.SearchDocumentsParams, documentPb.SearchDocumentsRequestJson>>,
  Ok<Conforms<T.SearchOrgUnitsParams, orgunitPb.SearchOrgUnitsRequestJson>>,
  Ok<Conforms<T.SearchSubjectsParams, corePb.SearchSubjectsRequestJson>>,
  Ok<Conforms<T.SearchThingsParams, thingPb.SearchThingsRequestJson>>,
  Ok<Conforms<T.UserRoleChangeResponse, corePb.GrantUserRoleResponseJson>>,
  Ok<Conforms<T.UserRoleChangeResponse, corePb.RevokeUserRoleResponseJson>>,
  Ok<SameEnum<T.FinalDisposition, retentionPb.FinalDispositionJson>>,
  Ok<Conforms<T.RetentionPolicy, retentionPb.RetentionPolicyJson>>,
  Ok<Conforms<T.ListRetentionPoliciesResponse, retentionPb.ListRetentionPoliciesResponseJson>>,
  Ok<Conforms<T.SubjectRetention, retentionPb.SubjectRetentionJson>>,
  Ok<Conforms<T.RetentionHold, retentionPb.RetentionHoldJson>>,
  Ok<Conforms<T.GetCaseRetentionResponse, retentionPb.GetCaseRetentionResponseJson>>,
  Ok<Conforms<T.RetentionHoldResponse, retentionPb.PlaceRetentionHoldResponseJson>>,
  Ok<Conforms<T.RetentionHoldResponse, retentionPb.ReleaseRetentionHoldResponseJson>>,
  Ok<Conforms<T.DueRetention, retentionPb.DueRetentionJson>>,
  Ok<Conforms<T.ListDueRetentionsResponse, retentionPb.ListDueRetentionsResponseJson>>,
  Ok<Conforms<T.ApplyCaseTypeRetentionResponse, retentionPb.ApplyCaseTypeRetentionResponseJson>>,
  Ok<SameEnum<T.DispositionStatus, retentionPb.DispositionStatusJson>>,
  Ok<SameEnum<T.BordereauStatus, retentionPb.BordereauStatusJson>>,
  Ok<SameEnum<T.DispositionDecision, retentionPb.DispositionDecisionJson>>,
  Ok<Conforms<T.DispositionItem, retentionPb.DispositionItemJson>>,
  Ok<Conforms<T.DispositionBordereau, retentionPb.DispositionBordereauJson>>,
  Ok<Conforms<T.DispositionBordereauResponse, retentionPb.CreateDispositionBordereauResponseJson>>,
  Ok<Conforms<T.DispositionBordereauResponse, retentionPb.GetDispositionBordereauResponseJson>>,
  Ok<Conforms<T.DispositionBordereauResponse, retentionPb.ExecuteDispositionBordereauResponseJson>>,
  Ok<Conforms<T.DispositionBordereauResponse, retentionPb.CancelDispositionBordereauResponseJson>>,
  Ok<Conforms<T.ListDispositionBordereauxResponse, retentionPb.ListDispositionBordereauxResponseJson>>,
  Ok<Conforms<T.ListMyDispositionApprovalsResponse, retentionPb.ListMyDispositionApprovalsResponseJson>>,
  Ok<Conforms<T.DecideDispositionItemResponse, retentionPb.DecideDispositionItemResponseJson>>,
  Ok<Conforms<T.RebuildTransferPackageResponse, retentionPb.RebuildTransferPackageResponseJson>>,
]
