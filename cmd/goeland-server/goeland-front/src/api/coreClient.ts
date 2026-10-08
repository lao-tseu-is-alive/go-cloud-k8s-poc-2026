import type {
  AuditEvent,
  BatchGetUsersResponse,
  GetCurrentUserResponse,
  ListRelationshipsResponse,
  RelationshipType,
  SearchUsersResponse,
  SubjectKind,
  SubjectRelationship,
  User,
} from './types'
/**
 * CoreService REST calls: relationship types, relationships, audit, subjects.
 */
import { apiFetch } from './client'

export async function listRelationshipTypes (
  opts: { onlyActive?: boolean, sourceKind?: SubjectKind, targetKind?: SubjectKind } = {},
): Promise<RelationshipType[]> {
  const res = await apiFetch<{ relationshipTypes?: RelationshipType[] }>('/api/relationship-types', {
    query: {
      onlyActive: opts.onlyActive,
      sourceKind: opts.sourceKind,
      targetKind: opts.targetKind,
    },
  })
  return res.relationshipTypes ?? []
}

/**
 * One page of the relationships of a subject (GLD-053), outgoing or incoming,
 * or both in one sorted list (GLD-056); orderBy is "<field>" or "<field> desc".
 */
export function listRelationshipsPage (
  subjectId: string,
  opts: { outgoing?: boolean, bothDirections?: boolean, relationshipTypeCode?: string, orderBy?: string, pageSize?: number, pageToken?: string },
): Promise<ListRelationshipsResponse> {
  return apiFetch<ListRelationshipsResponse>(`/api/subjects/${encodeURIComponent(subjectId)}/relationships`, {
    query: {
      outgoing: opts.outgoing,
      bothDirections: opts.bothDirections,
      relationshipTypeCode: opts.relationshipTypeCode,
      orderBy: opts.orderBy,
      pageSize: opts.pageSize,
      pageToken: opts.pageToken,
    },
  })
}

/** Every relationship of one type and direction (for short lists such as the members of a unit). */
export async function listAllRelationships (
  subjectId: string,
  opts: { outgoing: boolean, relationshipTypeCode: string },
): Promise<SubjectRelationship[]> {
  const all: SubjectRelationship[] = []
  let pageToken: string | undefined
  do {
    const res = await listRelationshipsPage(subjectId, { ...opts, pageSize: 200, pageToken })
    all.push(...(res.relationships ?? []))
    pageToken = res.nextPageToken || undefined
  } while (pageToken)
  return all
}

export async function listAuditEvents (subjectId: string, pageSize = 50): Promise<AuditEvent[]> {
  const res = await apiFetch<{ events?: AuditEvent[] }>(
    `/api/subjects/${encodeURIComponent(subjectId)}/audit`,
    { query: { pageSize } },
  )
  return res.events ?? []
}

export async function linkSubjects (
  sourceSubjectId: string,
  targetSubjectId: string,
  relationshipTypeCode: string,
  roleDetail?: string,
): Promise<SubjectRelationship> {
  const res = await apiFetch<{ relationship?: SubjectRelationship }>('/api/relationships', {
    method: 'POST',
    body: { sourceSubjectId, targetSubjectId, relationshipTypeCode, roleDetail },
  })
  return res.relationship as SubjectRelationship
}

export function unlinkSubjects (relationshipId: string, reason: string): Promise<unknown> {
  return apiFetch(`/api/relationships/${encodeURIComponent(relationshipId)}`, {
    method: 'DELETE',
    query: { reason },
  })
}

/** Ends an open relationship (business end of validity); validTo defaults to now server-side. */
export async function endRelationship (
  relationshipId: string,
  reason: string,
  validTo?: string,
): Promise<SubjectRelationship> {
  const res = await apiFetch<{ relationship?: SubjectRelationship }>(
    `/api/relationships/${encodeURIComponent(relationshipId)}/end`,
    { method: 'POST', body: { reason, validTo } },
  )
  return res.relationship as SubjectRelationship
}

/** The authenticated caller: recorded profile, admin flag and scopes. */
export function getCurrentUser (): Promise<GetCurrentUserResponse> {
  return apiFetch<GetCurrentUserResponse>('/api/me')
}

/** Internal users whose name or e-mail contains query, by name. */
export async function searchUsers (query: string, pageSize = 20, signal?: AbortSignal): Promise<User[]> {
  const res = await apiFetch<SearchUsersResponse>('/api/users/search', { query: { query, pageSize }, signal })
  return res.users ?? []
}

/** Resolves operator ids (createdBy, actorUserId, ...) to users; unknown ids are absent. */
export async function batchGetUsers (userIds: string[]): Promise<User[]> {
  const res = await apiFetch<BatchGetUsersResponse>('/api/users:batchGet', { query: { userIds } })
  return res.users ?? []
}
