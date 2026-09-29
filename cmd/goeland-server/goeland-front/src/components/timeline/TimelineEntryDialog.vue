<script setup lang="ts">
  import type { SubjectRef, TimelineEntry, TimelineEntryType, TimelineVisibility } from '@/api/types'
  import type { CitedDocument, EntrySubmission } from '@/components/timeline/timelineForm'
  import { computed, nextTick, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import SubjectPicker from '@/components/core/SubjectPicker.vue'
  import { entryTypeStyle, fromLocalInput, MAX_BODY_LENGTH, OPERATOR_ENTRY_TYPES, toLocalInput, VISIBILITIES } from '@/components/timeline/timelineForm'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { maxLength, required } from '@/utils/validation'

  // Creates a draft, edits one, or prepares the correction of an immutable
  // entry (prefilled from it). Documents are chosen with the subject picker.
  const props = defineProps<{ mode: 'create' | 'edit' | 'correct', entry?: TimelineEntry, busy?: boolean }>()
  const open = defineModel<boolean>({ default: false })
  const emit = defineEmits<{ submit: [submission: EntrySubmission] }>()

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()

  const form = ref<{ validate: () => Promise<{ valid: boolean }> } | null>(null)
  const entryType = ref<TimelineEntryType>('TIMELINE_ENTRY_TYPE_COMMENT')
  const title = ref('')
  const body = ref('')
  const occurredAt = ref('')
  const visibility = ref<TimelineVisibility>('TIMELINE_VISIBILITY_CASE_PARTICIPANTS')
  const reason = ref('')
  const documents = ref<CitedDocument[]>([])
  const pickerId = ref<string | undefined>()

  const typeItems = computed(() => OPERATOR_ENTRY_TYPES.map(value => ({ value, title: enumLabel('TimelineEntryType', value) })))
  const visibilityItems = computed(() => VISIBILITIES.map(value => ({ value, title: enumLabel('TimelineVisibility', value) })))
  const heading = computed(() => t(`timeline.dialog.${props.mode}`))

  function initialType (): TimelineEntryType {
    const source = props.entry?.entryType
    return source && OPERATOR_ENTRY_TYPES.includes(source) ? source : 'TIMELINE_ENTRY_TYPE_COMMENT'
  }

  function reset () {
    const source = props.mode === 'create' ? undefined : props.entry
    entryType.value = initialType()
    title.value = source?.title ?? ''
    body.value = source?.body ?? ''
    // A correction is recorded now; an edited draft keeps its business date.
    occurredAt.value = toLocalInput(props.mode === 'edit' ? source?.occurredAt : undefined)
    visibility.value = source?.visibility ?? 'TIMELINE_VISIBILITY_CASE_PARTICIPANTS'
    reason.value = ''
    documents.value = (source?.documents ?? []).map(d => ({ id: d.documentId, label: d.documentLabel ?? d.documentId }))
    pickerId.value = undefined
  }

  watch(open, isOpen => {
    if (isOpen) reset()
  })

  async function addDocument (subject: SubjectRef) {
    if (!documents.value.some(d => d.id === subject.id)) {
      documents.value.push({ id: subject.id, label: subject.displayLabel })
    }
    await nextTick()
    pickerId.value = undefined
  }

  function removeDocument (id: string) {
    documents.value = documents.value.filter(d => d.id !== id)
  }

  async function submit () {
    const result = await form.value?.validate()
    if (!result?.valid) return
    emit('submit', {
      content: {
        entryType: entryType.value,
        title: title.value.trim(),
        body: body.value.trim(),
        visibility: visibility.value,
        occurredAt: fromLocalInput(occurredAt.value),
      },
      documents: documents.value,
      reason: reason.value.trim() || undefined,
    })
  }
</script>

<template>
  <v-dialog v-model="open" max-width="720" scrollable>
    <v-card>
      <v-card-title>{{ heading }}</v-card-title>

      <v-card-text>
        <v-alert
          v-if="mode === 'correct'"
          class="mb-4"
          density="compact"
          type="info"
          variant="tonal"
        >
          {{ t('timeline.dialog.correctHint') }}
        </v-alert>

        <v-form ref="form" @submit.prevent="submit">
          <v-row dense>
            <v-col cols="12" sm="6">
              <v-select v-model="entryType" :items="typeItems" :label="t('timeline.fields.type')">
                <template #selection="{ item }">
                  <v-icon class="mr-2" :color="entryTypeStyle(item.value).color" :icon="entryTypeStyle(item.value).icon" size="small" />
                  {{ item.title }}
                </template>
              </v-select>
            </v-col>

            <v-col cols="12" sm="6">
              <v-text-field
                v-model="occurredAt"
                :hint="t('timeline.fields.occurredAtHint')"
                :label="t('timeline.fields.occurredAt')"
                persistent-hint
                type="datetime-local"
              />
            </v-col>
          </v-row>

          <v-text-field v-model="title" :label="t('timeline.fields.title')" :rules="[maxLength(t, 500)]" />

          <v-textarea
            v-model="body"
            auto-grow
            counter
            :label="t('timeline.fields.body')"
            :maxlength="MAX_BODY_LENGTH"
            rows="4"
            :rules="[required(t), maxLength(t, MAX_BODY_LENGTH)]"
          />

          <v-select
            v-model="visibility"
            :hint="t('timeline.fields.visibilityHint')"
            :items="visibilityItems"
            :label="t('timeline.fields.visibility')"
            persistent-hint
          />

          <div class="text-subtitle-2 mt-4 mb-1">{{ t('timeline.fields.documents') }}</div>
          <p class="text-caption text-medium-emphasis mb-2">{{ t('timeline.fields.documentsHint') }}</p>

          <div v-if="documents.length > 0" class="d-flex flex-wrap ga-2 mb-2">
            <v-chip
              v-for="doc in documents"
              :key="doc.id"
              closable
              prepend-icon="mdi-file-document-outline"
              size="small"
              @click:close="removeDocument(doc.id)"
            >
              {{ doc.label }}
            </v-chip>
          </div>

          <SubjectPicker v-model="pickerId" kind="SUBJECT_KIND_DOCUMENT" :label="t('timeline.fields.addDocument')" @picked="addDocument" />

          <v-text-field v-if="mode === 'edit'" v-model="reason" class="mt-2" :label="t('finalize.reason')" />
        </v-form>
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="open = false">{{ t('actions.common.cancel') }}</v-btn>
        <v-btn color="primary" :loading="busy" variant="flat" @click="submit">{{ t('actions.common.save') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
