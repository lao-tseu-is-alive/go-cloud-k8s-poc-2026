<script setup lang="ts">
  import type { SubjectRef, User } from '@/api/types'
  import type { AssigneeChoice } from '@/components/task/taskForm'
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import SubjectPicker from '@/components/core/SubjectPicker.vue'
  import UserPicker from '@/components/core/UserPicker.vue'

  // Chooses who a task is assigned to: an internal user (searched by name or
  // e-mail, bound to the operator id), a live org unit, or nobody.
  const model = defineModel<AssigneeChoice>({ required: true })
  const { t } = useI18n()
  const unitPick = ref<string | undefined>()

  function setKind (kind: AssigneeChoice['kind']) {
    model.value = { kind }
  }

  function pickUser (user?: User) {
    model.value = user ? { kind: 'user', userId: user.id, label: user.displayName || user.email || user.id } : { kind: 'user' }
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

    <UserPicker
      v-if="model.kind === 'user'"
      :label="t('tasks.assignee.pickUser')"
      :model-value="model.userId"
      @picked="pickUser"
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
