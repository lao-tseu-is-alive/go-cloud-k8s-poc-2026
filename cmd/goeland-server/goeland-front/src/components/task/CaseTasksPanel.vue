<script setup lang="ts">
  import type { Task, TaskStatus } from '@/api/types'
  import { ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { listCaseTasks } from '@/api/taskClient'
  import TaskDialogs from '@/components/task/TaskDialogs.vue'
  import { PENDING_STATUSES } from '@/components/task/taskForm'
  import TaskTable from '@/components/task/TaskTable.vue'
  import { useApiErrors } from '@/composables/useApiErrors'

  // The tasks of a case: pending ones by default, all on demand. Reports the
  // pending count (a case cannot close while tasks are open) and asks the page
  // to reload after a change (a completion adds a timeline entry).
  const props = defineProps<{ caseId: string, canEdit?: boolean, reloadKey?: number }>()
  const emit = defineEmits<{ changed: [], open: [count: number] }>()

  const { t } = useI18n()
  const { report } = useApiErrors()
  const tasks = ref<Task[]>([])
  const loading = ref(false)
  const showAll = ref(false)
  const dialogs = ref<InstanceType<typeof TaskDialogs> | null>(null)

  async function load () {
    loading.value = true
    try {
      const statuses: TaskStatus[] | undefined = showAll.value ? undefined : PENDING_STATUSES
      const res = await listCaseTasks(props.caseId, { statuses, pageSize: 200 })
      tasks.value = res.tasks ?? []
      emit('open', res.openCount ?? 0)
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  async function changed () {
    await load()
    emit('changed')
  }

  watch([() => props.caseId, () => props.reloadKey, showAll], load, { immediate: true })
</script>

<template>
  <div>
    <div class="d-flex flex-wrap align-center ga-2 mb-2">
      <v-switch
        v-model="showAll"
        color="primary"
        density="compact"
        hide-details
        :label="t('tasks.showAll')"
      />

      <v-spacer />

      <v-btn
        v-if="canEdit"
        color="primary"
        prepend-icon="mdi-plus"
        size="small"
        variant="flat"
        @click="dialogs?.openCreate()"
      >
        {{ t('tasks.actions.create') }}
      </v-btn>
    </div>

    <v-progress-linear v-if="loading" class="mb-2" color="primary" indeterminate />
    <p v-if="!loading && tasks.length === 0" class="text-medium-emphasis">{{ t(showAll ? 'tasks.emptyAll' : 'tasks.empty') }}</p>

    <TaskTable
      v-else
      :can-edit="canEdit"
      :tasks="tasks"
      @assign="dialogs?.openAssign($event)"
      @edit="dialogs?.openEdit($event)"
      @history="dialogs?.openHistory($event)"
      @move="(task, move) => dialogs?.openMove(task, move)"
    />

    <TaskDialogs ref="dialogs" :case-id="caseId" @changed="changed" />
  </div>
</template>
