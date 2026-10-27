/**
 * The transactional outbox (GLD-028), administrators only: its state and the
 * replay of the events whose delivery was given up.
 */
import type { OutboxStatus } from './types'
import { apiFetch } from './client'

export function getOutboxStatus (): Promise<OutboxStatus> {
  return apiFetch<OutboxStatus>('/api/outbox')
}

/** Replays the listed failed events, or every failed one; returns how many. */
export async function replayOutboxEvents (target: { eventIds: string[] } | { allFailed: true }): Promise<number> {
  const res = await apiFetch<{ replayed?: number }>('/api/outbox:replay', { method: 'POST', body: target })
  return res.replayed ?? 0
}
