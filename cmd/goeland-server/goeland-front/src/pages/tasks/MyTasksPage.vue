<script setup lang="ts">
  import type { Task, TaskStatus } from '@/api/types'
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { listMyTasks } from '@/api/taskClient'
  import TaskDialogs from '@/components/task/TaskDialogs.vue'
  import { TASK_STATUSES } from '@/components/task/taskForm'
  import TaskTable from '@/components/task/TaskTable.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'

  // The caller's tasks across cases, earliest deadline first: assigned to them
  // and, optionally, to the units they belong to.
  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const { report } = useApiErrors()
  const PAGE_SIZE = 50

  const tasks = ref<Task[]>([])
  const total = ref(0)
  const nextToken = ref('')
  const loading = ref(false)
  const includeUnits = ref(true)
  const statuses = ref<TaskStatus[]>([])
  const dialogs = ref<InstanceType<typeof TaskDialogs> | null>(null)

  const overdue = computed(() => tasks.value.filter(task => task.overdue).length)

  async function load (append = false) {
    loading.value = true
    try {
      const res = await listMyTasks({
        statuses: statuses.value.length > 0 ? statuses.value : undefined,
        includeUnits: includeUnits.value || undefined,
        pageSize: PAGE_SIZE,
        pageToken: append ? nextToken.value : undefined,
      })
      tasks.value = append ? [...tasks.value, ...(res.tasks ?? [])] : (res.tasks ?? [])
      total.value = res.totalSize ?? 0
      nextToken.value = res.nextPageToken ?? ''
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  watch([includeUnits, statuses], () => load(), { immediate: true })
</script>

<template>
  <v-container fluid>
    <div class="d-flex flex-wrap align-center ga-2 mb-4">
      <h1 class="text-h5">{{ t('tasks.mine.title') }}</h1>
      <v-chip size="small">{{ t('tasks.mine.count', { n: total }) }}</v-chip>
      <v-chip v-if="overdue > 0" color="error" prepend-icon="mdi-alarm" size="small">{{ t('tasks.mine.overdue', { n: overdue }) }}</v-chip>
    </div>

    <v-card>
      <v-card-text>
        <div class="d-flex flex-wrap align-center ga-4 mb-2">
          <v-chip-group v-model="statuses" column filter multiple>
            <v-chip
              v-for="status in TASK_STATUSES"
              :key="status"
              size="small"
              :value="status"
              variant="outlined"
            >
              {{ enumLabel('TaskStatus', status) }}
            </v-chip>
          </v-chip-group>

          <v-switch
            v-model="includeUnits"
            color="primary"
            density="compact"
            hide-details
            :label="t('tasks.mine.includeUnits')"
          />
        </div>

        <v-progress-linear v-if="loading" class="mb-2" color="primary" indeterminate />
        <p v-if="!loading && tasks.length === 0" class="text-medium-emphasis">{{ t('tasks.mine.empty') }}</p>

        <TaskTable
          v-else
          can-edit
          show-case
          :tasks="tasks"
          @assign="dialogs?.openAssign($event)"
          @edit="dialogs?.openEdit($event)"
          @history="dialogs?.openHistory($event)"
          @move="(task, move) => dialogs?.openMove(task, move)"
          @respond="dialogs?.openRespond($event)"
        />

        <div v-if="nextToken" class="d-flex justify-center mt-2">
          <v-btn :loading="loading" variant="text" @click="load(true)">{{ t('actions.common.loadMore') }} ({{ tasks.length }} / {{ total }})</v-btn>
        </div>
      </v-card-text>
    </v-card>

    <TaskDialogs ref="dialogs" @changed="load()" />
  </v-container>
</template>
