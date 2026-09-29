import type { TimelineEntry, TimelineEntryContent, TimelineEntryType, TimelineVisibility } from '@/api/types'

/** A document cited by the entry being edited. */
export interface CitedDocument { id: string, label: string }

/** What the entry dialog hands back; the timeline panel performs the API calls. */
export interface EntrySubmission {
  content: TimelineEntryContent
  documents: CitedDocument[]
  reason?: string
}

/**
 * Entry types an operator may write (timeline.EntryType.OperatorCreatable):
 * SYSTEM entries come from the server and AI proposals from the AI component.
 */
export const OPERATOR_ENTRY_TYPES: TimelineEntryType[] = [
  'TIMELINE_ENTRY_TYPE_COMMENT',
  'TIMELINE_ENTRY_TYPE_OPINION',
  'TIMELINE_ENTRY_TYPE_DECISION',
  'TIMELINE_ENTRY_TYPE_REQUEST',
  'TIMELINE_ENTRY_TYPE_RESPONSE',
  'TIMELINE_ENTRY_TYPE_VALIDATION',
]

/** Types offered as a list filter. */
export const FILTER_ENTRY_TYPES: TimelineEntryType[] = [...OPERATOR_ENTRY_TYPES, 'TIMELINE_ENTRY_TYPE_SYSTEM']

export const VISIBILITIES: TimelineVisibility[] = [
  'TIMELINE_VISIBILITY_CASE_PARTICIPANTS',
  'TIMELINE_VISIBILITY_INTERNAL',
  'TIMELINE_VISIBILITY_RESTRICTED',
]

/** Body length limit (timeline.MaxBodyLength). */
export const MAX_BODY_LENGTH = 20_000

const TYPE_STYLES: Partial<Record<TimelineEntryType, { icon: string, color: string }>> = {
  TIMELINE_ENTRY_TYPE_COMMENT: { icon: 'mdi-comment-text-outline', color: 'blue-grey' },
  TIMELINE_ENTRY_TYPE_OPINION: { icon: 'mdi-account-voice', color: 'indigo' },
  TIMELINE_ENTRY_TYPE_DECISION: { icon: 'mdi-gavel', color: 'deep-purple' },
  TIMELINE_ENTRY_TYPE_REQUEST: { icon: 'mdi-email-arrow-right-outline', color: 'orange-darken-2' },
  TIMELINE_ENTRY_TYPE_RESPONSE: { icon: 'mdi-email-arrow-left-outline', color: 'teal' },
  TIMELINE_ENTRY_TYPE_VALIDATION: { icon: 'mdi-check-decagram', color: 'green' },
  TIMELINE_ENTRY_TYPE_SYSTEM: { icon: 'mdi-cog-outline', color: 'grey' },
  TIMELINE_ENTRY_TYPE_AI_PROPOSAL: { icon: 'mdi-robot-outline', color: 'pink' },
}

/** Icon and dot color of an entry type. */
export function entryTypeStyle (type?: TimelineEntryType): { icon: string, color: string } {
  return (type && TYPE_STYLES[type]) || { icon: 'mdi-circle-small', color: 'grey' }
}

/** Only a draft changes (validated, locked and withdrawn entries are immutable). */
export function isDraft (entry: TimelineEntry): boolean {
  return entry.status === 'TIMELINE_ENTRY_STATUS_DRAFT'
}

/** A validated or locked entry that nothing corrects yet may be corrected. */
export function isCorrectable (entry: TimelineEntry): boolean {
  const frozen = entry.status === 'TIMELINE_ENTRY_STATUS_VALIDATED' || entry.status === 'TIMELINE_ENTRY_STATUS_LOCKED'
  return frozen && !entry.correctedByEntryId
}

/** Chip color of an entry status. */
export function entryStatusColor (entry: TimelineEntry): string {
  switch (entry.status) {
    case 'TIMELINE_ENTRY_STATUS_DRAFT': { return 'warning'
    }
    case 'TIMELINE_ENTRY_STATUS_VALIDATED': { return 'success'
    }
    case 'TIMELINE_ENTRY_STATUS_LOCKED': { return 'info'
    }
    default: { return 'grey'
    }
  }
}

/** A metadata value as display text (only strings are shown). */
export function metadataText (metadata: Record<string, unknown> | undefined, key: string): string {
  const value = metadata?.[key]
  return typeof value === 'string' ? value : ''
}
