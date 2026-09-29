import type { OrgUnit, OrgUnitInput, OrgUnitNode } from '@/api/types'
import { getOrgUnit } from '@/api/orgUnitClient'

/** Edit model of a unit (the dialog binds it; the parent is chosen apart). */
export interface OrgUnitForm {
  orgUnitTypeCode: string
  abbreviation: string
  label: string
  description: string
  email: string
  parentId?: string
  /** Display label of the chosen parent; empty for a root. */
  parentLabel: string
}

export function emptyOrgUnitForm (parent?: { id: string, label: string }): OrgUnitForm {
  return {
    orgUnitTypeCode: '',
    abbreviation: '',
    label: '',
    description: '',
    email: '',
    parentId: parent?.id,
    parentLabel: parent?.label ?? '',
  }
}

export function orgUnitToForm (unit: OrgUnit, parentLabel: string): OrgUnitForm {
  return {
    orgUnitTypeCode: unit.orgUnitType?.code ?? '',
    abbreviation: unit.abbreviation ?? '',
    label: unit.label,
    description: unit.description ?? '',
    email: unit.email ?? '',
    parentId: unit.parentId || undefined,
    parentLabel,
  }
}

/** The create / update request of a form (update replaces every field). */
export function formToInput (form: OrgUnitForm, reason?: string): OrgUnitInput {
  return {
    orgUnitTypeCode: form.orgUnitTypeCode,
    abbreviation: form.abbreviation.trim(),
    label: form.label.trim(),
    description: form.description.trim(),
    email: form.email.trim(),
    parentId: form.parentId || undefined,
    reason: reason?.trim() || undefined,
  }
}

/** A unit as shown in lists: "Label (ABBR)", like its subject label. */
export function nodeLabel (node: { label: string, abbreviation?: string }): string {
  return node.abbreviation ? `${node.label} (${node.abbreviation})` : node.label
}

/** A node of the tree view. */
export interface TreeItem {
  id: string
  title: string
  node: OrgUnitNode
  children?: TreeItem[]
}

/** Builds the tree of the flat list (server order kept); orphans become roots. */
export function buildTree (nodes: OrgUnitNode[]): TreeItem[] {
  const items = new Map<string, TreeItem>(nodes.map(n => [n.id, { id: n.id, title: nodeLabel(n), node: n }]))
  const roots: TreeItem[] = []
  for (const n of nodes) {
    const item = items.get(n.id) as TreeItem
    const parent = n.parentId ? items.get(n.parentId) : undefined
    if (parent) {
      parent.children = [...(parent.children ?? []), item]
    } else {
      roots.push(item)
    }
  }
  return roots
}

// Units rarely change during a session: one request per unit, shared by every label.
const labelCache = new Map<string, Promise<string>>()

/** The display label of a unit, fetched once (the id itself when it cannot be read). */
export function orgUnitLabel (id: string): Promise<string> {
  let label = labelCache.get(id)
  if (!label) {
    label = getOrgUnit(id).then(res => res.orgUnit?.subjectRef?.displayLabel ?? id, () => id)
    labelCache.set(id, label)
  }
  return label
}

/** Forgets a cached label after the unit changed. */
export function forgetOrgUnitLabel (id: string): void {
  labelCache.delete(id)
}
