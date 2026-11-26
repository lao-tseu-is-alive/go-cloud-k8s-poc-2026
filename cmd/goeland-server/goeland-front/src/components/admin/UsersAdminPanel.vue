<script setup lang="ts">
  import type { AppRole, User } from '@/api/types'
  import type { ListSort } from '@/utils/listSort'
  import { computed, onMounted, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { linkSubjects, registerUser, searchUsers } from '@/api/coreClient'
  import { listAppRoles } from '@/api/roleClient'
  import SortableHeader from '@/components/core/SortableHeader.vue'
  import SubjectPicker from '@/components/core/SubjectPicker.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useUiStore } from '@/stores/ui'
  import { formatDateTime } from '@/utils/formatters'
  import { sortRows } from '@/utils/listSort'
  import { required } from '@/utils/validation'

  // The users Goéland knows (GLD-091): everyone who signed in at least once,
  // with their application roles, administrators first to spot; a name opens
  // the user's page (units, groups, roles, GLD-093). An employee who has not
  // signed in yet is pre-registered here, optionally with their unit.
  const LIMIT = 200
  const { t } = useI18n()
  const { report } = useApiErrors()
  const router = useRouter()
  const ui = useUiStore()

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

  // Pre-registration of an employee (GLD-093).
  const registering = ref(false)
  const form = ref({ userId: '', displayName: '', email: '', unit: undefined as string | undefined, reason: '' })
  const busy = ref(false)
  const validId = (v: string) => /^[1-9]\d{0,18}$/.test(v.trim()) || t('users.userIdHint')
  const canRegister = computed(() => validId(form.value.userId) === true && !!form.value.displayName.trim() && !!form.value.reason.trim())

  function openRegister () {
    form.value = { userId: '', displayName: '', email: '', unit: undefined, reason: '' }
    registering.value = true
  }

  async function register () {
    busy.value = true
    try {
      const f = form.value
      const user = await registerUser({ userId: f.userId.trim(), displayName: f.displayName.trim(), email: f.email.trim() || undefined, reason: f.reason.trim() })
      if (f.unit && user.subjectId) {
        await linkSubjects(user.subjectId, f.unit, 'USER_MEMBER_OF_ORG_UNIT')
      }
      ui.notify(t('users.registered'), 'success')
      registering.value = false
      void router.push(`/admin/users/${user.id}`)
    } catch (error) {
      report(error)
    } finally {
      busy.value = false
    }
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
      <v-spacer />
      <v-btn color="primary" prepend-icon="mdi-account-plus" variant="flat" @click="openRegister">{{ t('users.register') }}</v-btn>
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
            <router-link :to="`/admin/users/${u.id}`">{{ u.displayName || u.id }}</router-link>

            <v-chip
              v-if="!u.firstSeenAt"
              class="ml-1"
              color="info"
              label
              size="x-small"
            >{{ t('users.pending') }}</v-chip>
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

          <td class="text-caption">{{ u.firstSeenAt ? formatDateTime(u.firstSeenAt) : '—' }}</td>
          <td class="text-caption">{{ u.lastSeenAt ? formatDateTime(u.lastSeenAt) : '—' }}</td>
        </tr>

        <tr v-if="!loading && shown.length === 0">
          <td class="text-medium-emphasis text-center py-6" colspan="5">{{ t('messages.common.noData') }}</td>
        </tr>
      </tbody>
    </v-table>

    <v-dialog v-model="registering" max-width="560">
      <v-card :title="t('users.registerTitle')">
        <v-card-text>
          <p class="text-body-2 text-medium-emphasis mb-3">{{ t('users.registerHint') }}</p>

          <v-text-field
            v-model="form.userId"
            :hint="t('users.userIdHint')"
            inputmode="numeric"
            :label="t('users.userId')"
            persistent-hint
            :rules="[validId]"
          />

          <v-text-field v-model="form.displayName" class="mt-2" :label="t('users.name')" :rules="[required(t)]" />
          <v-text-field v-model="form.email" :label="t('users.email')" type="email" />
          <SubjectPicker v-model="form.unit" kind="SUBJECT_KIND_ORG_UNIT" :label="t('users.unit')" />
          <v-text-field v-model="form.reason" class="mt-2" :label="t('finalize.reason')" :rules="[required(t)]" />
        </v-card-text>

        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="registering = false">{{ t('actions.common.cancel') }}</v-btn>

          <v-btn
            color="primary"
            :disabled="!canRegister"
            :loading="busy"
            variant="flat"
            @click="register"
          >{{ t('users.register') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<style scoped>
  .users-search {
    flex: 1 1 16rem;
    max-width: 28rem;
  }
</style>
