/**
 * Application roles stored in Goéland (GLD-047): catalogue, holders, a user's
 * assignments, and grant / revoke (administrators only, with a reason).
 */
import type { AppRole, ListAppRolesResponse, ListRoleHoldersResponse, ListUserRolesResponse, User, UserRole, UserRoleChangeResponse } from './types'
import { apiFetch } from './client'

/** The application role catalogue, by code. */
export async function listAppRoles (): Promise<AppRole[]> {
  const res = await apiFetch<ListAppRolesResponse>('/api/app-roles')
  return res.roles ?? []
}

/** The users currently holding a role, by name. */
export async function listRoleHolders (roleCode: string): Promise<User[]> {
  const res = await apiFetch<ListRoleHoldersResponse>(`/api/app-roles/${encodeURIComponent(roleCode)}/holders`)
  return res.users ?? []
}

/** A user's role assignments, newest first; with the revoked ones when includeRevoked. */
export async function listUserRoles (userId: string, includeRevoked = false): Promise<UserRole[]> {
  const res = await apiFetch<ListUserRolesResponse>(`/api/users/${encodeURIComponent(userId)}/roles`, { query: { includeRevoked } })
  return res.roles ?? []
}

/** Grants a role to a user who has signed in at least once. */
export function grantUserRole (userId: string, roleCode: string, reason: string): Promise<UserRoleChangeResponse> {
  return apiFetch<UserRoleChangeResponse>(`/api/users/${encodeURIComponent(userId)}/roles`, { method: 'POST', body: { roleCode, reason } })
}

/** Revokes a held role (kept as history); the last administrator is refused. */
export function revokeUserRole (userId: string, roleCode: string, reason: string): Promise<UserRoleChangeResponse> {
  return apiFetch<UserRoleChangeResponse>(
    `/api/users/${encodeURIComponent(userId)}/roles/${encodeURIComponent(roleCode)}/revoke`,
    { method: 'POST', body: { reason } },
  )
}
