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
import { listOrgUnitTypes } from '@/api/orgUnitClient'
import { FINAL_DISPOSITIONS, listRetentionPolicies } from '@/api/retentionClient'
import { listTaskTypes } from '@/api/taskClient'
import { listThingTypes } from '@/api/thingClient'

/** confidentiality is a level 0-5 (from 2 a grant is needed to read). */
export type FieldKind = 'text' | 'textarea' | 'subjectKind' | 'boolean' | 'confidentiality' | 'choice' | 'number'

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
  /** The values of a 'choice' field. */
  choices?: string[]
  /** The enum whose labels name the choices (useI18nEnum); the values themselves otherwise. */
  choiceEnum?: string
  /** Loads the choices from the server (another catalogue) instead of `choices`. */
  loadChoices?: () => Promise<ChoiceItem[]>
  /** The choice may be emptied (sent as ''), e.g. a case type without retention policy. */
  clearable?: boolean
  /** The bounds of a 'number' field. */
  min?: number
  /** The value of a new entry. */
  initial?: unknown
}

/** One choice of a 'choice' field. */
export interface ChoiceItem {
  value: string
  title: string
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
    fields: [
      LABEL,
      { key: 'businessRefNamespace', kind: 'text', max: 32, column: true },
      { key: 'defaultConfidentialityLevel', kind: 'confidentiality', column: true },
      // The retention policy given to its cases when closed (GLD-032).
      { key: 'defaultRetentionPolicyCode', kind: 'choice', column: true, clearable: true, loadChoices: retentionPolicyChoices },
      DESCRIPTION,
    ],
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
      // The role read from the subject on screen (GLD-063, utils/roleHeadings.ts).
      { key: 'roleLabel', kind: 'text', max: 100, column: true },
      { key: 'roleLabelPlural', kind: 'text', max: 100 },
      { key: 'roleEnd', kind: 'choice', choices: ['ROLE_END_TARGET', 'ROLE_END_SOURCE'], choiceEnum: 'RoleEnd', initial: 'ROLE_END_TARGET' },
      { key: 'rolePreposition', kind: 'choice', choices: ['de', 'à', 'par', 'sur', 'dans', 'avec', 'pour'], initial: 'de' },
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
  {
    // New thing types are generic; parcel and building types are seeded.
    catalogue: 'thing_type',
    fields: [LABEL, DESCRIPTION],
    list: async () => (await listThingTypes(false)) as unknown as CatalogueEntry[],
  },
  {
    // New unit types are ordered after the seven seeded ones.
    catalogue: 'org_unit_type',
    fields: [LABEL, DESCRIPTION],
    list: async () => (await listOrgUnitTypes(false)) as unknown as CatalogueEntry[],
  },
  {
    catalogue: 'task_type',
    fields: [LABEL, DESCRIPTION],
    list: async () => (await listTaskTypes(false)) as unknown as CatalogueEntry[],
  },
  {
    // The retention schedule (GLD-032); a change applies to cases closed afterwards.
    catalogue: 'retention_policy',
    fields: [
      LABEL,
      { key: 'durationYears', kind: 'number', required: true, min: 0, max: 200, column: true, initial: 10 },
      { key: 'finalDisposition', kind: 'choice', choices: [...FINAL_DISPOSITIONS], choiceEnum: 'FinalDisposition', column: true, initial: 'FINAL_DISPOSITION_ARCHIVE' },
      { key: 'legalBasis', kind: 'textarea', max: 2000 },
      DESCRIPTION,
    ],
    list: async () => (await listRetentionPolicies(false)) as unknown as CatalogueEntry[],
  },
]

/** The active retention policies, as choices. */
async function retentionPolicyChoices (): Promise<ChoiceItem[]> {
  return (await listRetentionPolicies(true)).map(p => ({ value: p.code, title: p.label }))
}

/** Confidentiality levels; from CONFIDENTIAL_LEVEL a subject is read only with a grant. */
export const CONFIDENTIALITY_LEVELS = [0, 1, 2, 3, 4, 5]
export const CONFIDENTIAL_LEVEL = 2

/** The code rule of new entries, mirroring the server. */
export const REFERENCE_CODE = /^[A-Z][A-Z0-9_]{1,99}$/
