<script setup lang="ts">
  import type { SubjectRef, User } from '@/api/types'
  import type { AssigneeChoice } from '@/components/task/taskForm'
  import { onBeforeUnmount, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { searchUsers } from '@/api/coreClient'
  import SubjectPicker from '@/components/core/SubjectPicker.vue'
  import { useApiErrors } from '@/composables/useApiErrors'

  // Chooses who a task is assigned to: an internal user (searched by name or
  // e-mail, bound to the operator id), a live org unit, or nobody.
  const model = defineModel<AssigneeChoice>({ required: true })
  const { t } = useI18n()
  const { report } = useApiErrors()
  const DEBOUNCE_MS = 250

  const users = ref<User[]>([])
  const userSearch = ref('')
  const loading = ref(false)
  const unitPick = ref<string | undefined>()
  let timer: ReturnType<typeof setTimeout> | undefined
  let controller: AbortController | undefined

  async function findUsers (query: string) {
    controller?.abort()
    controller = new AbortController()
    loading.value = true
    try {
      users.value = await searchUsers(query.trim(), 20, controller.signal)
    } catch (error) {
      if (!(error instanceof DOMException && error.name === 'AbortError')) report(error)
    } finally {
      loading.value = false
    }
  }

  watch(userSearch, term => {
    clearTimeout(timer)
    timer = setTimeout(() => void findUsers(term ?? ''), DEBOUNCE_MS)
  })
  watch(() => model.value.kind, kind => {
    if (kind === 'user' && users.value.length === 0) void findUsers('')
  }, { immediate: true })
  onBeforeUnmount(() => {
    clearTimeout(timer)
    controller?.abort()
  })

  function setKind (kind: AssigneeChoice['kind']) {
    model.value = { kind }
  }

  function pickUser (userId?: string) {
    const user = users.value.find(u => u.id === userId)
    model.value = userId ? { kind: 'user', userId, label: user?.displayName || user?.email || userId } : { kind: 'user' }
  }

  function pickUnit (unit: SubjectRef) {
    model.value = { kind: 'unit', orgUnitId: unit.id, label: unit.displayLabel }
  }
</script>

<template>
  <div>
    <div class="text-subtitle-2 mb-1">{{ t('tasks.fields.assignee') }}</div>

    <v-btn-toggle
      class="mb-2"
      color="primary"
      density="compact"
      mandatory
      :model-value="model.kind"
      variant="outlined"
      @update:model-value="setKind"
    >
      <v-btn value="none">{{ t('tasks.assignee.none') }}</v-btn>
      <v-btn prepend-icon="mdi-account-circle-outline" value="user">{{ t('tasks.assignee.user') }}</v-btn>
      <v-btn prepend-icon="mdi-sitemap-outline" value="unit">{{ t('tasks.assignee.unit') }}</v-btn>
    </v-btn-toggle>

    <v-chip v-if="model.kind !== 'none' && model.label" class="ml-2 mb-2" size="small">{{ model.label }}</v-chip>

    <v-autocomplete
      v-if="model.kind === 'user'"
      v-model:search="userSearch"
      clearable
      :item-title="(u: User) => u.displayName || u.email || u.id"
      item-value="id"
      :items="users"
      :label="t('tasks.assignee.pickUser')"
      :loading="loading"
      :model-value="model.userId"
      no-filter
      prepend-inner-icon="mdi-account-search-outline"
      @update:model-value="pickUser"
    />

    <SubjectPicker
      v-if="model.kind === 'unit'"
      v-model="unitPick"
      kind="SUBJECT_KIND_ORG_UNIT"
      :label="t('tasks.assignee.pickUnit')"
      @picked="pickUnit"
    />
  </div>
</template>
