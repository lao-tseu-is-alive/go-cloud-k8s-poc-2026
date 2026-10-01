/**
 * Grants and security groups (GLD-048): the caller's effective level on a
 * subject, a subject's grants (set, change, revoke) and the groups with their
 * members. Every change needs a level the server checks (FULL_CONTROL to grant,
 * MANAGE on a group to change its members).
 */
import type { Access, AccessGrant, GetMyAccessResponse, GrantChangeResponse, GranteeKind, GroupResponse, ListGrantsResponse, ListGroupsResponse, Permission, SecurityGroup } from './types'
import { apiFetch } from './client'

const subjectPath = (id: string) => `/api/subjects/${encodeURIComponent(id)}`
const groupPath = (id: string) => `/api/groups/${encodeURIComponent(id)}`

/** The caller's effective level on a subject. */
export async function getMyAccess (subjectId: string): Promise<Access | undefined> {
  const res = await apiFetch<GetMyAccessResponse>(`${subjectPath(subjectId)}/access`)
  return res.access
}

/** A subject's grants, current first; with the history when includeRevoked. */
export async function listGrants (subjectId: string, includeRevoked = false): Promise<AccessGrant[]> {
  const res = await apiFetch<ListGrantsResponse>(`${subjectPath(subjectId)}/grants`, { query: { includeRevoked } })
  return res.grants ?? []
}

/** Gives or changes the grant of a grantee (the old level is kept as history). */
export function setGrant (subjectId: string, granteeKind: GranteeKind, granteeId: string, level: Permission, reason: string): Promise<GrantChangeResponse> {
  return apiFetch<GrantChangeResponse>(`${subjectPath(subjectId)}/grants`, { method: 'POST', body: { granteeKind, granteeId, level, reason } })
}

/** Revokes a grant; the last FULL_CONTROL of a subject is refused. */
export function revokeGrant (grantId: string, reason: string): Promise<GrantChangeResponse> {
  return apiFetch<GrantChangeResponse>(`/api/grants/${encodeURIComponent(grantId)}/revoke`, { method: 'POST', body: { reason } })
}

/** The security groups whose name contains query, by name. */
export async function listGroups (query = '', includeArchived = false): Promise<SecurityGroup[]> {
  const res = await apiFetch<ListGroupsResponse>('/api/groups', { query: { query, includeArchived } })
  return res.groups ?? []
}

/** A group and its current members. */
export function getGroup (id: string): Promise<GroupResponse> {
  return apiFetch<GroupResponse>(groupPath(id))
}

/** Creates a group; its creator gets FULL_CONTROL on it. */
export function createGroup (name: string, description: string): Promise<GroupResponse> {
  return apiFetch<GroupResponse>('/api/groups', { method: 'POST', body: { name, description } })
}

/** Renames or redescribes a live group. */
export function updateGroup (id: string, name: string, description: string, reason: string): Promise<GroupResponse> {
  return apiFetch<GroupResponse>(groupPath(id), { method: 'PATCH', body: { name, description, reason } })
}

/** Archives a group; its grants stop applying. */
export function archiveGroup (id: string, reason: string): Promise<GroupResponse> {
  return apiFetch<GroupResponse>(`${groupPath(id)}/archive`, { method: 'POST', body: { reason } })
}

/** Adds an internal user to a group. */
export function addGroupMember (id: string, userId: string): Promise<GroupResponse> {
  return apiFetch<GroupResponse>(`${groupPath(id)}/members`, { method: 'POST', body: { userId } })
}

/** Removes a member (the membership is kept as history). */
export function removeGroupMember (id: string, userId: string, reason: string): Promise<GroupResponse> {
  return apiFetch<GroupResponse>(`${groupPath(id)}/members/${encodeURIComponent(userId)}/remove`, { method: 'POST', body: { reason } })
}
