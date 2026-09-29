/** Subject helpers shared by every page that shows a subject reference. */
import type { SubjectKind, SubjectRef } from '@/api/types'

// Same icons as the navigation, so a subject reads the same everywhere.
const KIND_ICONS: Partial<Record<SubjectKind, string>> = {
  SUBJECT_KIND_CASE: 'mdi-briefcase-outline',
  SUBJECT_KIND_DOCUMENT: 'mdi-file-document-outline',
  SUBJECT_KIND_ACTOR: 'mdi-account-multiple',
  SUBJECT_KIND_THING: 'mdi-map-marker-outline',
  SUBJECT_KIND_ORG_UNIT: 'mdi-sitemap-outline',
  SUBJECT_KIND_USER: 'mdi-account-circle-outline',
}

// SPA pages per subject kind; USER has none yet.
const KIND_ROUTES: Partial<Record<SubjectKind, string>> = {
  SUBJECT_KIND_CASE: '/cases',
  SUBJECT_KIND_DOCUMENT: '/documents',
  SUBJECT_KIND_ACTOR: '/actors',
  SUBJECT_KIND_THING: '/things',
  SUBJECT_KIND_ORG_UNIT: '/org-units',
}

export function kindIcon (kind?: SubjectKind): string {
  return (kind && KIND_ICONS[kind]) ?? 'mdi-shape-outline'
}

/** The SPA route of a subject's detail page, or undefined when its kind has none. */
export function subjectRoute (ref?: SubjectRef): string | undefined {
  const base = ref?.kind ? KIND_ROUTES[ref.kind] : undefined
  return base && ref?.id ? `${base}/${encodeURIComponent(ref.id)}` : undefined
}
