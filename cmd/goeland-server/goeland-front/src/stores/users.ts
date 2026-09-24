/**
 * Internal users directory (setup style): resolves operator ids found in
 * governance and audit (createdBy, actorUserId, ...) to names.
 *
 * Components ask for ids one by one; the store batches every id requested in
 * the same tick into BatchGetUsers calls (at most 200 ids each) and caches the
 * answers, including "unknown" (null), for the lifetime of the page.
 */
import type { User } from '@/api/types'
import { defineStore } from 'pinia'
import { reactive } from 'vue'
import { batchGetUsers } from '@/api/coreClient'

const BATCH_SIZE = 200

export const useUsersStore = defineStore('users', () => {
  // id → user, null when the server does not know the id.
  const known = reactive(new Map<string, User | null>())
  const pending = new Set<string>()
  let scheduled = false

  async function flush (): Promise<void> {
    scheduled = false
    const ids = [...pending]
    pending.clear()
    for (let i = 0; i < ids.length; i += BATCH_SIZE) {
      const chunk = ids.slice(i, i + BATCH_SIZE)
      try {
        const users = await batchGetUsers(chunk)
        for (const id of chunk) {
          known.set(id, users.find(u => u.id === id) ?? null)
        }
      } catch {
        // Leave them unresolved: the raw id stays displayed, a later view retries.
      }
    }
  }

  /** Schedules the resolution of id (no-op when known or already pending). */
  function request (id?: string): void {
    if (!id || known.has(id) || pending.has(id)) {
      return
    }
    pending.add(id)
    if (!scheduled) {
      scheduled = true
      queueMicrotask(() => void flush())
    }
  }

  /** The resolved user, undefined while unknown or unresolved. */
  function get (id?: string): User | undefined {
    return (id && known.get(id)) || undefined
  }

  /** Records a user already at hand (e.g. the signed-in user). */
  function remember (user?: User): void {
    if (user?.id) {
      known.set(user.id, user)
    }
  }

  function clear (): void {
    known.clear()
    pending.clear()
  }

  return { request, get, remember, clear }
})
