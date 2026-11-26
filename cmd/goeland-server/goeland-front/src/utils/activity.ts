/** The caller's activity on a subject in « Mon tableau de bord » (GLD-088). */
import type { SubjectActivity } from '@/api/types'

/** The latest thing the caller did on a subject: changed it or consulted it, and when. */
export interface LastActivity {
  kind: 'changed' | 'viewed'
  at: string
}

/** The latest of the caller's last change and last consultation (a change wins a tie). */
export function lastActivity (activity?: SubjectActivity): LastActivity | null {
  const changed = activity?.lastChangedAt
  const viewed = activity?.lastViewedAt
  if (!changed && !viewed) {
    return null
  }
  if (changed && (!viewed || Date.parse(changed) >= Date.parse(viewed))) {
    return { kind: 'changed', at: changed }
  }
  return { kind: 'viewed', at: viewed ?? '' }
}

/** Indexes the activities of a search response by subject id. */
export function activityById (activities?: SubjectActivity[]): Record<string, SubjectActivity> {
  const byId: Record<string, SubjectActivity> = {}
  for (const a of activities ?? []) {
    if (a.subjectId) {
      byId[a.subjectId] = a
    }
  }
  return byId
}
