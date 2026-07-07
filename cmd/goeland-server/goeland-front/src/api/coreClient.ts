import type {
  AuditEvent,
  RelationshipType,
  SubjectKind,
  SubjectRelationship,
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

export async function listRelationships (
  subjectId: string,
  opts: { outgoing?: boolean, relationshipTypeCode?: string } = {},
): Promise<SubjectRelationship[]> {
  const res = await apiFetch<{ relationships?: SubjectRelationship[] }>(
    `/api/subjects/${encodeURIComponent(subjectId)}/relationships`,
    { query: { outgoing: opts.outgoing, relationshipTypeCode: opts.relationshipTypeCode } },
  )
  return res.relationships ?? []
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
