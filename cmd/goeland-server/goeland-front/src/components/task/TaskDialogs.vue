<script setup lang="ts">
  import type { TaskMove } from '@/api/taskClient'
  import type { Task, TaskType } from '@/api/types'
  import type { AssigneeChoice } from '@/components/task/taskForm'
  import { computed, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { assignTask, createTask, getTask, listTaskTypes, moveTask, updateTask } from '@/api/taskClient'
  import UserLabel from '@/components/core/UserLabel.vue'
  import AssigneePicker from '@/components/task/AssigneePicker.vue'
  import { assigneeFields, assigneeOf, moveNeedsText } from '@/components/task/taskForm'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useUiStore } from '@/stores/ui'
  import { fromLocalInput, toLocalInput } from '@/utils/dateInput'
  import { formatDateTime } from '@/utils/formatters'
  import { maxLength, required } from '@/utils/validation'

  // Every task dialog (create, edit, assign, status moves, history) behind one
  // component: the parent opens them through the exposed functions and reloads
  // on "changed".
  const props = defineProps<{ caseId?: string }>()
  const emit = defineEmits<{ changed: [] }>()

  const { t } = useI18n()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const current = ref<Task | null>(null)
  const busy = ref(false)
  const types = ref<TaskType[]>([])

  // content dialog (create / edit)
  const contentOpen = ref(false)
  const contentMode = ref<'create' | 'edit'>('create')
  const contentForm = ref<{ validate: () => Promise<{ valid: boolean }> } | null>(null)
  const typeCode = ref('')
  const title = ref('')
  const description = ref('')
  const dueAt = ref('')
  const assignee = ref<AssigneeChoice>({ kind: 'none' })
  const reason = ref('')

  // assign, move and history dialogs
  const assignOpen = ref(false)
  const moveOpen = ref(false)
  const move = ref<TaskMove>('start')
  const moveText = ref('')
  const historyOpen = ref(false)

  const typeItems = computed(() => types.value
    .filter(ty => ty.isActive || ty.code === current.value?.taskType?.code)
    .map(ty => ({ value: ty.code, title: ty.label || ty.code })))
  const textRequired = computed(() => moveNeedsText(move.value))

  async function loadTypes () {
    try {
      types.value = await listTaskTypes(false)
    } catch (error) {
      report(error)
    }
  }

  function openCreate () {
    current.value = null
    contentMode.value = 'create'
    typeCode.value = ''
    title.value = ''
    description.value = ''
    dueAt.value = ''
    assignee.value = { kind: 'none' }
    contentOpen.value = true
    void loadTypes()
  }

  function openEdit (task: Task) {
    current.value = task
    contentMode.value = 'edit'
    typeCode.value = task.taskType?.code ?? ''
    title.value = task.title
    description.value = task.description ?? ''
    dueAt.value = task.dueAt ? toLocalInput(task.dueAt) : ''
    reason.value = ''
    contentOpen.value = true
    void loadTypes()
  }

  function openAssign (task: Task) {
    current.value = task
    assignee.value = assigneeOf(task)
    reason.value = ''
    assignOpen.value = true
  }

  function openMove (task: Task, next: TaskMove) {
    current.value = task
    move.value = next
    moveText.value = ''
    moveOpen.value = true
  }

  async function openHistory (task: Task) {
    try {
      current.value = await getTask(task.id)
      historyOpen.value = true
    } catch (error) {
      report(error)
    }
  }

  // run performs one API call with the busy flag, feedback and reload.
  async function run (call: () => Promise<unknown>, message: string, close: () => void) {
    busy.value = true
    try {
      await call()
      ui.notify(t(message), 'success')
      close()
      emit('changed')
    } catch (error) {
      report(error)
    } finally {
      busy.value = false
    }
  }

  async function saveContent () {
    const result = await contentForm.value?.validate()
    if (!result?.valid) return
    const content = { taskTypeCode: typeCode.value, title: title.value.trim(), description: description.value.trim(), dueAt: fromLocalInput(dueAt.value) }
    const task = current.value
    const call = contentMode.value === 'create'
      ? () => createTask(props.caseId ?? '', { ...content, ...assigneeFields(assignee.value) })
      : () => updateTask(task?.id ?? '', { ...content, reason: reason.value.trim() || undefined })
    await run(call, `tasks.messages.${contentMode.value}`, () => (contentOpen.value = false))
  }

  async function saveAssign () {
    const id = current.value?.id ?? ''
    await run(() => assignTask(id, { ...assigneeFields(assignee.value), reason: reason.value.trim() || undefined }), 'tasks.messages.assign', () => (assignOpen.value = false))
  }

  async function saveMove () {
    if (textRequired.value && !moveText.value.trim()) return
    const id = current.value?.id ?? ''
    await run(() => moveTask(id, move.value, moveText.value.trim() || undefined), `tasks.messages.${move.value}`, () => (moveOpen.value = false))
  }

  defineExpose({ openCreate, openEdit, openAssign, openMove, openHistory })
</script>

<template>
  <v-dialog v-model="contentOpen" max-width="640" scrollable>
    <v-card>
      <v-card-title>{{ t(`tasks.dialog.${contentMode}`) }}</v-card-title>

      <v-card-text>
        <v-form ref="contentForm" @submit.prevent="saveContent">
          <v-select v-model="typeCode" :items="typeItems" :label="t('tasks.fields.type')" :rules="[required(t)]" />
          <v-text-field v-model="title" :label="t('tasks.fields.title')" :rules="[required(t), maxLength(t, 500)]" />

          <v-textarea
            v-model="description"
            auto-grow
            :label="t('tasks.fields.description')"
            rows="2"
            :rules="[maxLength(t, 4000)]"
          />

          <v-text-field v-model="dueAt" clearable :label="t('tasks.fields.dueAt')" type="datetime-local" />
          <AssigneePicker v-if="contentMode === 'create'" v-model="assignee" />
          <v-text-field v-else v-model="reason" :label="t('finalize.reason')" />
        </v-form>
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="contentOpen = false">{{ t('actions.common.cancel') }}</v-btn>
        <v-btn color="primary" :loading="busy" variant="flat" @click="saveContent">{{ t('actions.common.save') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="assignOpen" max-width="560">
    <v-card>
      <v-card-title>{{ t('tasks.actions.assign') }}</v-card-title>

      <v-card-text>
        <p class="text-body-2 mb-3">{{ current?.title }}</p>
        <AssigneePicker v-model="assignee" />
        <v-text-field v-model="reason" class="mt-2" :label="t('finalize.reason')" />
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="assignOpen = false">{{ t('actions.common.cancel') }}</v-btn>
        <v-btn color="primary" :loading="busy" variant="flat" @click="saveAssign">{{ t('actions.common.confirm') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="moveOpen" max-width="520">
    <v-card>
      <v-card-title>{{ t(`tasks.actions.${move}`) }}</v-card-title>

      <v-card-text>
        <p class="text-body-2 mb-3">{{ current?.title }}</p>

        <v-textarea
          v-if="move !== 'start'"
          v-model="moveText"
          auto-grow
          :label="t(move === 'complete' ? 'tasks.fields.note' : 'finalize.reason')"
          rows="2"
          :rules="textRequired ? [required(t)] : []"
        />

        <p v-if="move === 'complete' || move === 'cancel'" class="text-caption text-medium-emphasis">{{ t('tasks.timelineHint') }}</p>
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="moveOpen = false">{{ t('actions.common.cancel') }}</v-btn>
        <v-btn :color="move === 'cancel' ? 'error' : 'primary'" :loading="busy" variant="flat" @click="saveMove">{{ t('actions.common.confirm') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="historyOpen" max-width="640" scrollable>
    <v-card>
      <v-card-title>{{ t('tasks.actions.history') }}</v-card-title>

      <v-card-text>
        <p class="text-body-2 mb-3">{{ current?.title }}</p>
        <p v-if="!current?.assignments?.length" class="text-medium-emphasis">{{ t('tasks.noHistory') }}</p>

        <v-timeline v-else align="start" density="compact" side="end">
          <v-timeline-item
            v-for="a in current.assignments"
            :key="a.id"
            :dot-color="a.endedAt ? 'grey' : 'primary'"
            size="x-small"
          >
            <div class="font-weight-medium">{{ a.assigneeLabel }}</div>

            <div class="text-caption text-medium-emphasis">
              {{ formatDateTime(a.assignedAt) }} · <UserLabel :id="a.assignedBy" />
              <span v-if="a.endedAt"> → {{ formatDateTime(a.endedAt) }}</span>
            </div>

            <div v-if="a.reason" class="text-caption">{{ a.reason }}</div>
          </v-timeline-item>
        </v-timeline>
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="historyOpen = false">{{ t('actions.common.close') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
