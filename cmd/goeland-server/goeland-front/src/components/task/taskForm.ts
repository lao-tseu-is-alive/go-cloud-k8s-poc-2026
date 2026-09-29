import type { TaskMove } from '@/api/taskClient'
import type { Task, TaskStatus } from '@/api/types'

/** Statuses a task can be filtered on, in lifecycle order. */
export const TASK_STATUSES: TaskStatus[] = ['TASK_STATUS_OPEN', 'TASK_STATUS_IN_PROGRESS', 'TASK_STATUS_DONE', 'TASK_STATUS_CANCELLED']

/** The pending statuses (a case cannot close while tasks are in them). */
export const PENDING_STATUSES: TaskStatus[] = ['TASK_STATUS_OPEN', 'TASK_STATUS_IN_PROGRESS']

export function isPending (task: Task): boolean {
  return !!task.status && PENDING_STATUSES.includes(task.status)
}

/**
 * The moves offered for a task, mirroring the server state machine
 * (task.moves); the server stays the authority.
 */
export function allowedMoves (task: Task): TaskMove[] {
  switch (task.status) {
    case 'TASK_STATUS_OPEN': { return ['start', 'complete', 'cancel']
    }
    case 'TASK_STATUS_IN_PROGRESS': { return ['complete', 'cancel']
    }
    case 'TASK_STATUS_DONE':
    case 'TASK_STATUS_CANCELLED': { return ['reopen']
    }
    default: { return []
    }
  }
}

/** Cancelling and reopening need a reason; completing takes an optional note. */
export function moveNeedsText (move: TaskMove): boolean {
  return move === 'cancel' || move === 'reopen'
}

export const MOVE_ICONS: Record<TaskMove, string> = {
  start: 'mdi-play-outline',
  complete: 'mdi-check-circle-outline',
  cancel: 'mdi-close-circle-outline',
  reopen: 'mdi-restore',
}

/** Chip color of a task status. */
export function taskStatusColor (status?: TaskStatus): string {
  switch (status) {
    case 'TASK_STATUS_OPEN': { return 'info'
    }
    case 'TASK_STATUS_IN_PROGRESS': { return 'primary'
    }
    case 'TASK_STATUS_DONE': { return 'success'
    }
    default: { return 'grey'
    }
  }
}

/** Who a task is (to be) assigned to, as edited in the dialogs. */
export interface AssigneeChoice {
  kind: 'none' | 'user' | 'unit'
  userId?: string
  orgUnitId?: string
  label?: string
}

export function assigneeOf (task?: Task): AssigneeChoice {
  if (task?.assigneeUserId) {
    return { kind: 'user', userId: task.assigneeUserId, label: task.assigneeLabel }
  }
  if (task?.assigneeOrgUnitId) {
    return { kind: 'unit', orgUnitId: task.assigneeOrgUnitId, label: task.assigneeLabel }
  }
  return { kind: 'none' }
}

/** The request fields of an assignee choice. */
export function assigneeFields (a: AssigneeChoice): { assigneeUserId?: string, assigneeOrgUnitId?: string } {
  if (a.kind === 'user') {
    return { assigneeUserId: a.userId }
  }
  if (a.kind === 'unit') {
    return { assigneeOrgUnitId: a.orgUnitId }
  }
  return {}
}
