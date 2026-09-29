<script setup lang="ts">
  import type { RecipientDraft } from '@/components/circulation/circulationForm'
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { createCirculation } from '@/api/circulationClient'
  import AssigneePicker from '@/components/task/AssigneePicker.vue'
  import { assigneeFields } from '@/components/task/taskForm'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useUiStore } from '@/stores/ui'
  import { fromLocalInput } from '@/utils/dateInput'
  import { maxLength, required } from '@/utils/validation'

  // Sends the case to recipients (users or units). Recipients sharing a step
  // answer in parallel; the next step opens once the previous one answered.
  const props = defineProps<{ caseId: string }>()
  const open = defineModel<boolean>({ default: false })
  const emit = defineEmits<{ created: [] }>()

  const { t } = useI18n()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const form = ref<{ validate: () => Promise<{ valid: boolean }> } | null>(null)
  const title = ref('')
  const message = ref('')
  const dueAt = ref('')
  const drafts = ref<RecipientDraft[]>([])
  const busy = ref(false)

  const chosen = computed(() => drafts.value.filter(d => d.assignee.kind !== 'none' && (d.assignee.userId || d.assignee.orgUnitId)))

  watch(open, isOpen => {
    if (!isOpen) return
    title.value = ''
    message.value = ''
    dueAt.value = ''
    drafts.value = [{ step: 1, assignee: { kind: 'unit' } }]
  })

  function addRecipient () {
    const last = drafts.value.at(-1)
    drafts.value.push({ step: last?.step ?? 1, assignee: { kind: 'unit' } })
  }

  function removeRecipient (index: number) {
    drafts.value.splice(index, 1)
  }

  async function submit () {
    const result = await form.value?.validate()
    if (!result?.valid || chosen.value.length === 0) return
    busy.value = true
    try {
      await createCirculation(props.caseId, {
        title: title.value.trim(),
        message: message.value.trim() || undefined,
        dueAt: fromLocalInput(dueAt.value),
        recipients: chosen.value.map(d => ({ step: d.step, ...assigneeFields(d.assignee) })),
      })
      ui.notify(t('circulations.messages.created'), 'success')
      open.value = false
      emit('created')
    } catch (error) {
      report(error)
    } finally {
      busy.value = false
    }
  }
</script>

<template>
  <v-dialog v-model="open" max-width="760" scrollable>
    <v-card>
      <v-card-title>{{ t('circulations.dialog.create') }}</v-card-title>

      <v-card-text>
        <v-form ref="form" @submit.prevent="submit">
          <v-text-field v-model="title" :label="t('circulations.fields.title')" :rules="[required(t), maxLength(t, 500)]" />

          <v-textarea
            v-model="message"
            auto-grow
            :label="t('circulations.fields.message')"
            rows="2"
            :rules="[maxLength(t, 4000)]"
          />

          <v-text-field v-model="dueAt" clearable :label="t('circulations.fields.dueAt')" type="datetime-local" />

          <div class="text-subtitle-2 mt-2">{{ t('circulations.fields.recipients') }}</div>
          <p class="text-caption text-medium-emphasis mb-2">{{ t('circulations.stepsHint') }}</p>

          <v-sheet
            v-for="(draft, index) in drafts"
            :key="index"
            border
            class="pa-3 mb-2"
            rounded
          >
            <div class="d-flex align-start ga-3">
              <v-text-field
                v-model.number="draft.step"
                density="compact"
                hide-details
                :label="t('circulations.fields.step')"
                min="1"
                style="max-width: 110px"
                type="number"
              />

              <AssigneePicker v-model="draft.assignee" class="flex-grow-1" />

              <v-btn
                :aria-label="t('circulations.actions.removeRecipient')"
                icon="mdi-close"
                size="small"
                :title="t('circulations.actions.removeRecipient')"
                variant="text"
                @click="removeRecipient(index)"
              />
            </div>
          </v-sheet>

          <v-btn prepend-icon="mdi-plus" size="small" variant="text" @click="addRecipient">{{ t('circulations.actions.addRecipient') }}</v-btn>
          <p v-if="chosen.length === 0" class="text-caption text-error">{{ t('circulations.recipientsRequired') }}</p>
        </v-form>
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="open = false">{{ t('actions.common.cancel') }}</v-btn>

        <v-btn
          color="primary"
          :disabled="chosen.length === 0"
          :loading="busy"
          variant="flat"
          @click="submit"
        >{{ t('circulations.actions.send') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
