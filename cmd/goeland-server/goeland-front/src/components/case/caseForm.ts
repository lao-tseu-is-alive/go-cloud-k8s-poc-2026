import type { CaseStatus } from '@/api/types'

/**
 * Allowed status transitions, mirroring casefile.CanTransition on the server
 * (the server stays the authority: this only decides which actions to offer).
 */
export const CASE_TRANSITIONS: Record<CaseStatus, CaseStatus[]> = {
  CASE_STATUS_UNSPECIFIED: [],
  CASE_STATUS_OPEN: ['CASE_STATUS_IN_PROGRESS', 'CASE_STATUS_SUSPENDED', 'CASE_STATUS_CLOSED'],
  CASE_STATUS_IN_PROGRESS: ['CASE_STATUS_OPEN', 'CASE_STATUS_SUSPENDED', 'CASE_STATUS_CLOSED'],
  CASE_STATUS_SUSPENDED: ['CASE_STATUS_OPEN', 'CASE_STATUS_IN_PROGRESS', 'CASE_STATUS_CLOSED'],
  CASE_STATUS_CLOSED: ['CASE_STATUS_OPEN'],
}

/** Statuses selectable as a search filter. */
export const CASE_STATUSES: CaseStatus[] = [
  'CASE_STATUS_OPEN',
  'CASE_STATUS_IN_PROGRESS',
  'CASE_STATUS_SUSPENDED',
  'CASE_STATUS_CLOSED',
]

/** Closing and reopening must be justified (casefile.RequiresReason). */
export function transitionNeedsReason (from: CaseStatus | undefined, to: CaseStatus): boolean {
  return to === 'CASE_STATUS_CLOSED' || from === 'CASE_STATUS_CLOSED'
}

/** Chip color of a status. */
export function caseStatusColor (status: CaseStatus | undefined): string {
  switch (status) {
    case 'CASE_STATUS_OPEN': { return 'info'
    }
    case 'CASE_STATUS_IN_PROGRESS': { return 'primary'
    }
    case 'CASE_STATUS_SUSPENDED': { return 'warning'
    }
    case 'CASE_STATUS_CLOSED': { return 'success'
    }
    default: { return 'grey'
    }
  }
}
