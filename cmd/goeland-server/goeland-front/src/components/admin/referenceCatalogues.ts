/**
 * Declarative description of the administered catalogues: which fields each has,
 * which are required or fixed at creation, and how to list its entries. The
 * generic ReferenceCatalogPanel renders any of them.
 */
import type { ReferenceCatalogue, SubjectKind } from '@/api/types'
import { listOrganizationCategories } from '@/api/actorClient'
import { listCaseTypes } from '@/api/caseClient'
import { listRelationshipTypes } from '@/api/coreClient'
import { listDocumentTypes } from '@/api/documentClient'

export type FieldKind = 'text' | 'textarea' | 'subjectKind' | 'boolean'

export interface CatalogueField {
  /** Proto3-JSON field name, also the i18n key under fields.reference. */
  key: string
  kind: FieldKind
  required?: boolean
  /** Set at creation only (immutable afterwards). */
  createOnly?: boolean
  max?: number
  /** Shown as a table column. */
  column?: boolean
}

/** Any catalogue entry, as proto3 JSON. */
export type CatalogueEntry = Record<string, unknown> & { code: string, label: string, isActive?: boolean }

export interface CatalogueConfig {
  catalogue: ReferenceCatalogue
  fields: CatalogueField[]
  list: () => Promise<CatalogueEntry[]>
}

const LABEL: CatalogueField = { key: 'label', kind: 'text', required: true, max: 200, column: true }
const DESCRIPTION: CatalogueField = { key: 'description', kind: 'textarea', max: 2000 }

export const SUBJECT_KINDS: SubjectKind[] = [
  'SUBJECT_KIND_CASE', 'SUBJECT_KIND_DOCUMENT', 'SUBJECT_KIND_THING', 'SUBJECT_KIND_ACTOR', 'SUBJECT_KIND_USER', 'SUBJECT_KIND_ORG_UNIT',
]

export const CATALOGUES: CatalogueConfig[] = [
  {
    catalogue: 'case_type',
    fields: [LABEL, { key: 'businessRefNamespace', kind: 'text', max: 32, column: true }, DESCRIPTION],
    list: async () => (await listCaseTypes(false)) as unknown as CatalogueEntry[],
  },
  {
    catalogue: 'relationship_type',
    fields: [
      LABEL,
      { key: 'sourceKind', kind: 'subjectKind', required: true, createOnly: true, column: true },
      { key: 'targetKind', kind: 'subjectKind', required: true, createOnly: true, column: true },
      { key: 'isDirected', kind: 'boolean', createOnly: true },
      { key: 'inverseLabel', kind: 'text', max: 200 },
      DESCRIPTION,
    ],
    list: async () => (await listRelationshipTypes({ onlyActive: false })) as unknown as CatalogueEntry[],
  },
  {
    catalogue: 'organization_category',
    fields: [LABEL],
    list: async () => (await listOrganizationCategories(false)) as unknown as CatalogueEntry[],
  },
  {
    catalogue: 'document_type',
    fields: [LABEL, { key: 'category', kind: 'text', max: 100, column: true }, DESCRIPTION],
    list: async () => (await listDocumentTypes(false)) as unknown as CatalogueEntry[],
  },
]

/** The code rule of new entries, mirroring the server. */
export const REFERENCE_CODE = /^[A-Z][A-Z0-9_]{1,99}$/
