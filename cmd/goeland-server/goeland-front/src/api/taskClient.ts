import type { CreateTaskRequest, ListTasksResponse, Task, TaskContent, TaskStatus, TaskType } from './types'
/**
 * TaskService REST calls (case tasks). Every mutation returns the task as
 * stored; the server is the authority on the allowed moves.
 */
import { apiFetch } from './client'

const caseTasksPath = (caseId: string) => `/api/cases/${encodeURIComponent(caseId)}/tasks`
const taskPath = (id: string) => `/api/tasks/${encodeURIComponent(id)}`

type TaskResponse = { task?: Task }

export function listCaseTasks (caseId: string, params: { statuses?: TaskStatus[], pageSize?: number, pageToken?: string } = {}): Promise<ListTasksResponse> {
  return apiFetch<ListTasksResponse>(caseTasksPath(caseId), { query: params as Record<string, unknown> })
}

/** The caller's tasks across cases; without statuses, the pending ones. */
export function listMyTasks (params: { statuses?: TaskStatus[], includeUnits?: boolean, pageSize?: number, pageToken?: string } = {}): Promise<ListTasksResponse> {
  return apiFetch<ListTasksResponse>('/api/tasks/mine', { query: params as Record<string, unknown> })
}

export async function getTask (id: string): Promise<Task> {
  const res = await apiFetch<TaskResponse>(taskPath(id))
  return res.task as Task
}

export async function createTask (caseId: string, req: CreateTaskRequest): Promise<Task> {
  const res = await apiFetch<TaskResponse>(caseTasksPath(caseId), { method: 'POST', body: req })
  return res.task as Task
}

export async function updateTask (id: string, req: TaskContent & { reason?: string }): Promise<Task> {
  const res = await apiFetch<TaskResponse>(taskPath(id), { method: 'PATCH', body: req })
  return res.task as Task
}

/** Assigns a user or a unit; both empty unassigns the task. */
export async function assignTask (id: string, req: { assigneeUserId?: string, assigneeOrgUnitId?: string, reason?: string }): Promise<Task> {
  const res = await apiFetch<TaskResponse>(`${taskPath(id)}/assign`, { method: 'POST', body: req })
  return res.task as Task
}

/** A lifecycle move; complete takes an optional note, cancel and reopen a required reason. */
export type TaskMove = 'start' | 'complete' | 'cancel' | 'reopen'

export async function moveTask (id: string, move: TaskMove, text?: string): Promise<Task> {
  const body = move === 'complete' ? { note: text } : { reason: text }
  const res = await apiFetch<TaskResponse>(`${taskPath(id)}/${move}`, { method: 'POST', body })
  return res.task as Task
}

export async function listTaskTypes (onlyActive = true): Promise<TaskType[]> {
  const res = await apiFetch<{ taskTypes?: TaskType[] }>('/api/task-types', { query: { onlyActive } })
  return res.taskTypes ?? []
}
