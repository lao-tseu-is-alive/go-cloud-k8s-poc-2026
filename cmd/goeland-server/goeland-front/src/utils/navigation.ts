/** The main navigation of the app bar and of the mobile drawer (one list, both places). */
export interface NavItem {
  to: string
  icon: string
  /** i18n key of the label. */
  label: string
  /** Shown to administrators only. */
  admin?: boolean
  /** A main module: keeps its label on a large screen where the others show their icon only. */
  primary?: boolean
}

export const NAV_ITEMS: readonly NavItem[] = [
  { to: '/cases', icon: 'mdi-briefcase-outline', label: 'nav.cases', primary: true },
  { to: '/documents', icon: 'mdi-file-document-multiple', label: 'nav.documents', primary: true },
  { to: '/things', icon: 'mdi-map-marker-outline', label: 'nav.things', primary: true },
  { to: '/actors', icon: 'mdi-account-multiple', label: 'nav.actors', primary: true },
  { to: '/tasks/mine', icon: 'mdi-checkbox-marked-circle-outline', label: 'nav.myTasks' },
  { to: '/org-units', icon: 'mdi-sitemap-outline', label: 'nav.orgUnits' },
  { to: '/groups', icon: 'mdi-account-group-outline', label: 'nav.groups' },
  { to: '/admin', icon: 'mdi-cog-outline', label: 'nav.admin', admin: true },
]

/** The entries a user sees: the administration only for administrators. */
export function navItemsFor (isAdmin: boolean): NavItem[] {
  return NAV_ITEMS.filter(item => !item.admin || isAdmin)
}
