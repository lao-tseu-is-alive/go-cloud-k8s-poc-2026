<script setup lang="ts">
  import type { AppRole, User } from '@/api/types'
  import type { ListSort } from '@/utils/listSort'
  import { computed, onMounted, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { searchUsers } from '@/api/coreClient'
  import { listAppRoles } from '@/api/roleClient'
  import SortableHeader from '@/components/core/SortableHeader.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { formatDateTime } from '@/utils/formatters'
  import { sortRows } from '@/utils/listSort'

  // The users Goéland knows (GLD-091): everyone who signed in at least once,
  // with their application roles, administrators first to spot. Roles are
  // granted and revoked in the « Rôles » tab.
  const LIMIT = 200
  const { t } = useI18n()
  const { report } = useApiErrors()

  const users = ref<User[]>([])
  const roles = ref<AppRole[]>([])
  const query = ref('')
  const onlyAdmins = ref(false)
  const loading = ref(false)
  const sort = ref<ListSort>()

  const shown = computed(() => sortRows(users.value.filter(u => !onlyAdmins.value || u.isAdmin), sort.value, (u, field) => {
    if (field === 'email') return u.email
    if (field === 'last_seen_at') return u.lastSeenAt
    if (field === 'first_seen_at') return u.firstSeenAt
    return u.displayName || u.id
  }))
  const adminCount = computed(() => users.value.filter(u => u.isAdmin).length)

  function roleLabel (code: string): string {
    return roles.value.find(r => r.code === code)?.label || code
  }

  let timer: ReturnType<typeof setTimeout> | undefined
  async function load () {
    loading.value = true
    try {
      users.value = await searchUsers(query.value.trim(), LIMIT)
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  watch(query, () => {
    clearTimeout(timer)
    timer = setTimeout(() => void load(), 300)
  })

  onMounted(async () => {
    void load()
    try {
      roles.value = await listAppRoles()
    } catch (error) {
      report(error)
    }
  })
</script>

<template>
  <div>
    <p class="text-body-2 text-medium-emphasis mb-3">{{ t('users.hint') }}</p>

    <div class="d-flex flex-wrap align-center ga-4 mb-2">
      <v-text-field
        v-model="query"
        class="users-search"
        clearable
        density="compact"
        hide-details
        :label="t('users.search')"
        prepend-inner-icon="mdi-magnify"
      />

      <v-switch
        v-model="onlyAdmins"
        color="primary"
        density="compact"
        hide-details
        :label="t('users.onlyAdmins')"
      />

      <v-chip size="small">{{ t('users.count', { n: shown.length, admins: adminCount }) }}</v-chip>
    </div>

    <v-progress-linear v-if="loading" class="mb-2" color="primary" indeterminate />

    <v-alert
      v-if="users.length >= LIMIT"
      class="mb-2"
      density="compact"
      type="info"
      variant="tonal"
    >{{ t('users.limited', { n: LIMIT }) }}</v-alert>

    <v-table density="compact">
      <thead>
        <tr>
          <SortableHeader v-model:sort="sort" field="name" :label="t('users.name')" />
          <SortableHeader v-model:sort="sort" field="email" :label="t('users.email')" />
          <th scope="col">{{ t('users.roles') }}</th>
          <SortableHeader v-model:sort="sort" field="first_seen_at" :label="t('users.firstSeen')" />
          <SortableHeader v-model:sort="sort" field="last_seen_at" :label="t('users.lastSeen')" />
        </tr>
      </thead>

      <tbody>
        <tr v-for="u in shown" :key="u.id">
          <td>
            <v-icon class="mr-1" :color="u.isAdmin ? 'warning' : undefined" :icon="u.isAdmin ? 'mdi-shield-account' : 'mdi-account-circle'" size="small" />
            {{ u.displayName || u.id }}
          </td>

          <td class="text-caption">{{ u.email || '—' }}</td>

          <td>
            <v-chip
              v-for="code in u.roles ?? []"
              :key="code"
              class="mr-1"
              :color="code === 'ADMIN' ? 'warning' : 'primary'"
              label
              size="x-small"
            >{{ roleLabel(code) }}</v-chip>

            <span v-if="(u.roles ?? []).length === 0" class="text-medium-emphasis">—</span>
          </td>

          <td class="text-caption">{{ formatDateTime(u.firstSeenAt) }}</td>
          <td class="text-caption">{{ formatDateTime(u.lastSeenAt) }}</td>
        </tr>

        <tr v-if="!loading && shown.length === 0">
          <td class="text-medium-emphasis text-center py-6" colspan="5">{{ t('messages.common.noData') }}</td>
        </tr>
      </tbody>
    </v-table>
  </div>
</template>

<style scoped>
  .users-search {
    flex: 1 1 16rem;
    max-width: 28rem;
  }
</style>
