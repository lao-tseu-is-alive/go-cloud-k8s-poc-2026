<script setup lang="ts">
  import type { SubjectRelationship } from '@/api/types'
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { endRelationship } from '@/api/coreClient'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useUiStore } from '@/stores/ui'

  // Ends a relationship in the business sense (the edge stays as history). Unlike
  // the pure LinkSubjectDialog it owns the API call, so every relationship table
  // gets the same behaviour; it emits `ended` once the server confirmed.
  const props = defineProps<{ relationship?: SubjectRelationship }>()
  const emit = defineEmits<{ ended: [rel: SubjectRelationship] }>()
  const open = defineModel<boolean>({ required: true })

  const { t } = useI18n()
  const { report } = useApiErrors()
  const ui = useUiStore()
  const endDate = ref('')
  const reason = ref('')
  const busy = ref(false)

  const label = computed(() => {
    const rel = props.relationship
    return rel ? `${rel.source?.displayLabel ?? ''} → ${rel.target?.displayLabel ?? ''}` : ''
  })

  watch(open, isOpen => {
    if (isOpen) {
      endDate.value = ''
      reason.value = ''
    }
  })

  // An empty date means "now" (server time); a date means its local midnight.
  function validTo (): string | undefined {
    return endDate.value ? new Date(`${endDate.value}T00:00:00`).toISOString() : undefined
  }

  async function submit () {
    if (!props.relationship) return
    busy.value = true
    try {
      const rel = await endRelationship(props.relationship.id, reason.value.trim(), validTo())
      ui.notify(t('messages.relationship.ended'), 'success')
      open.value = false
      emit('ended', rel)
    } catch (error) {
      report(error)
    } finally {
      busy.value = false
    }
  }
</script>

<template>
  <v-dialog v-model="open" max-width="520">
    <v-card>
      <v-card-title>{{ t('actions.relationship.end') }}</v-card-title>

      <v-card-text>
        <p class="mb-1">{{ label }}</p>
        <p class="text-caption text-medium-emphasis mb-4">{{ t('messages.relationship.endHint') }}</p>

        <v-text-field
          v-model="endDate"
          clearable
          :hint="t('messages.relationship.endDateHint')"
          :label="t('fields.relationship.valid_to')"
          persistent-hint
          type="date"
        />

        <v-textarea
          v-model="reason"
          class="mt-2"
          counter="2000"
          :label="t('fields.relationship.end_reason')"
          rows="2"
        />
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="open = false">{{ t('actions.common.cancel') }}</v-btn>

        <v-btn color="warning" :loading="busy" variant="flat" @click="submit">
          {{ t('actions.relationship.end') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
