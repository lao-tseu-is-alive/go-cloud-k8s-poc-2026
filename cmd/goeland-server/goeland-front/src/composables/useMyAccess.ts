import type { Access, Permission } from '@/api/types'
import type { Ref } from 'vue'
import { computed, ref, watch } from 'vue'
import { getMyAccess } from '@/api/accessClient'

/** Rank of each level, so levels compare (NONE < READ < ... < FULL_CONTROL). */
export const LEVEL_RANK: Record<Permission, number> = {
  PERMISSION_UNSPECIFIED: 0,
  PERMISSION_NONE: 0,
  PERMISSION_READ: 1,
  PERMISSION_CONTRIBUTE: 2,
  PERMISSION_MANAGE: 3,
  PERMISSION_FULL_CONTROL: 4,
}

/**
 * The caller's effective level on a subject (GLD-048), reloaded when the id
 * changes. The server checks every call; this only hides the actions that
 * would be refused. Unknown while loading: nothing is offered.
 */
export function useMyAccess (subjectId: Ref<string | undefined>) {
  const access = ref<Access>()

  async function reload () {
    const id = subjectId.value
    access.value = id ? await getMyAccess(id).catch(() => undefined) : undefined
  }

  const rank = computed(() => LEVEL_RANK[access.value?.level ?? 'PERMISSION_NONE'])
  const atLeast = (level: Permission) => rank.value >= LEVEL_RANK[level]
  const canContribute = computed(() => atLeast('PERMISSION_CONTRIBUTE'))
  const canManage = computed(() => atLeast('PERMISSION_MANAGE'))
  const hasFullControl = computed(() => atLeast('PERMISSION_FULL_CONTROL'))

  watch(subjectId, reload, { immediate: true })
  return { access, reload, canContribute, canManage, hasFullControl }
}
