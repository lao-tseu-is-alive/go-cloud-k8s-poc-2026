<script setup lang="ts">
  import type { TaskMove } from '@/api/taskClient'
  import type { Task } from '@/api/types'
  import { useI18n } from 'vue-i18n'
  import UserLabel from '@/components/core/UserLabel.vue'
  import { allowedMoves, isPending, MOVE_ICONS, taskStatusColor } from '@/components/task/taskForm'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { formatDate } from '@/utils/formatters'

  // A list of tasks with their assignee, deadline (overdue highlighted) and the
  // actions their status allows. showCase adds the case column ("my tasks").
  defineProps<{ tasks: Task[], showCase?: boolean, canEdit?: boolean }>()
  const emit = defineEmits<{
    edit: [task: Task]
    assign: [task: Task]
    move: [task: Task, move: TaskMove]
    history: [task: Task]
  }>()

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
</script>

<template>
  <v-table density="compact">
    <thead>
      <tr>
        <th scope="col">{{ t('tasks.fields.title') }}</th>
        <th v-if="showCase" scope="col">{{ t('tasks.fields.case') }}</th>
        <th scope="col">{{ t('tasks.fields.assignee') }}</th>
        <th scope="col">{{ t('tasks.fields.dueAt') }}</th>
        <th scope="col">{{ t('tasks.fields.status') }}</th>
        <th scope="col" />
      </tr>
    </thead>

    <tbody>
      <tr v-for="task in tasks" :key="task.id" :class="{ 'task-closed': !isPending(task) }">
        <td>
          <div class="font-weight-medium">{{ task.title }}</div>
          <div class="text-caption text-medium-emphasis">{{ task.taskType?.label ?? task.taskType?.code }}</div>
        </td>

        <td v-if="showCase">
          <router-link class="task-link" :to="`/cases/${encodeURIComponent(task.caseId)}`">{{ task.caseLabel || task.caseId }}</router-link>
        </td>

        <td>
          <span v-if="task.assigneeLabel">
            <v-icon :icon="task.assigneeOrgUnitId ? 'mdi-sitemap-outline' : 'mdi-account-circle-outline'" size="small" />
            {{ task.assigneeLabel }}
          </span>

          <span v-else class="text-medium-emphasis">{{ t('tasks.assignee.none') }}</span>
        </td>

        <td>
          <v-chip v-if="task.overdue" color="error" prepend-icon="mdi-alarm" size="x-small">{{ formatDate(task.dueAt) }}</v-chip>
          <span v-else>{{ formatDate(task.dueAt) }}</span>
        </td>

        <td>
          <v-chip :color="taskStatusColor(task.status)" label size="x-small">{{ enumLabel('TaskStatus', task.status) }}</v-chip>

          <div v-if="task.completedBy || task.cancelledBy" class="text-caption text-medium-emphasis">
            <UserLabel :id="task.completedBy || task.cancelledBy" />
          </div>
        </td>

        <td class="text-no-wrap text-right">
          <template v-if="canEdit">
            <v-btn
              v-for="move in allowedMoves(task)"
              :key="move"
              :aria-label="t(`tasks.actions.${move}`)"
              :icon="MOVE_ICONS[move]"
              size="small"
              :title="t(`tasks.actions.${move}`)"
              variant="text"
              @click="emit('move', task, move)"
            />

            <v-btn
              v-if="isPending(task)"
              :aria-label="t('tasks.actions.assign')"
              icon="mdi-account-switch-outline"
              size="small"
              :title="t('tasks.actions.assign')"
              variant="text"
              @click="emit('assign', task)"
            />

            <v-btn
              v-if="isPending(task)"
              :aria-label="t('tasks.actions.edit')"
              icon="mdi-pencil-outline"
              size="small"
              :title="t('tasks.actions.edit')"
              variant="text"
              @click="emit('edit', task)"
            />
          </template>

          <v-btn
            :aria-label="t('tasks.actions.history')"
            icon="mdi-history"
            size="small"
            :title="t('tasks.actions.history')"
            variant="text"
            @click="emit('history', task)"
          />
        </td>
      </tr>
    </tbody>
  </v-table>
</template>

<style scoped>
  .task-closed td {
    color: rgb(var(--v-theme-on-surface), 0.6);
  }

  .task-link {
    color: rgb(var(--v-theme-primary));
    text-decoration: none;
  }

  .task-link:hover,
  .task-link:focus-visible {
    text-decoration: underline;
  }
</style>
