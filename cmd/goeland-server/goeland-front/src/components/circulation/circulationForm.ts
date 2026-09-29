import type { CirculationRecipient, CirculationResponse, CirculationStatus } from '@/api/types'
import type { AssigneeChoice } from '@/components/task/taskForm'

/** The answers a recipient can give, in the order of the spec (v2 §29). */
export const RESPONSES: CirculationResponse[] = [
  'CIRCULATION_RESPONSE_FAVORABLE',
  'CIRCULATION_RESPONSE_UNFAVORABLE',
  'CIRCULATION_RESPONSE_COMMENT',
  'CIRCULATION_RESPONSE_NOT_CONCERNED',
  'CIRCULATION_RESPONSE_NEED_MORE_INFO',
]

/** Answers that need a text (mirrors circulation.Response.NeedsText). */
export function responseNeedsText (r?: CirculationResponse): boolean {
  return r === 'CIRCULATION_RESPONSE_COMMENT' || r === 'CIRCULATION_RESPONSE_NEED_MORE_INFO'
}

export function responseColor (r?: CirculationResponse): string {
  switch (r) {
    case 'CIRCULATION_RESPONSE_FAVORABLE': { return 'success'
    }
    case 'CIRCULATION_RESPONSE_UNFAVORABLE': { return 'error'
    }
    case 'CIRCULATION_RESPONSE_NEED_MORE_INFO': { return 'warning'
    }
    default: { return 'info'
    }
  }
}

export function circulationStatusColor (s?: CirculationStatus): string {
  switch (s) {
    case 'CIRCULATION_STATUS_OPEN': { return 'primary'
    }
    case 'CIRCULATION_STATUS_COMPLETED': { return 'success'
    }
    default: { return 'grey'
    }
  }
}

/** A recipient being added to a new circulation. */
export interface RecipientDraft {
  step: number
  assignee: AssigneeChoice
}

/** Recipients grouped by step; the server returns them by step, so the order is kept. */
export function byStep (recipients: CirculationRecipient[]): { step: number, recipients: CirculationRecipient[] }[] {
  const groups = new Map<number, CirculationRecipient[]>()
  for (const rec of recipients) {
    const step = rec.step ?? 1
    groups.set(step, [...(groups.get(step) ?? []), rec])
  }
  return [...groups.entries()].map(([step, list]) => ({ step, recipients: list }))
}
