/** Shared presentation of grantees and access levels (AccessPanel, case type defaults). */
import type { GranteeKind, Permission } from '@/api/types'

/** The levels a grant gives, lowest first. */
export const LEVELS: Permission[] = ['PERMISSION_READ', 'PERMISSION_CONTRIBUTE', 'PERMISSION_MANAGE', 'PERMISSION_FULL_CONTROL']

export const LEVEL_COLORS: Partial<Record<Permission, string>> = {
  PERMISSION_READ: 'grey', PERMISSION_CONTRIBUTE: 'info', PERMISSION_MANAGE: 'primary', PERMISSION_FULL_CONTROL: 'deep-purple',
}

export const GRANTEE_ICONS: Record<GranteeKind, string> = {
  GRANTEE_KIND_UNSPECIFIED: 'mdi-help-circle-outline',
  GRANTEE_KIND_USER: 'mdi-account-circle-outline',
  GRANTEE_KIND_GROUP: 'mdi-account-group-outline',
  GRANTEE_KIND_ORG_UNIT: 'mdi-sitemap-outline',
  GRANTEE_KIND_CREATOR_UNITS: 'mdi-account-arrow-up-outline',
}
