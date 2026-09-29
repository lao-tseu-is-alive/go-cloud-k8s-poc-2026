<script setup lang="ts">
  import type { CirculationRecipient, CirculationResponse } from '@/api/types'
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { respondToCirculation } from '@/api/circulationClient'
  import { responseNeedsText, RESPONSES } from '@/components/circulation/circulationForm'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { useUiStore } from '@/stores/ui'
  import { maxLength, required } from '@/utils/validation'

  // Records the answer of one recipient; the server completes its task, writes
  // the RESPONSE entry and opens the next step or completes the circulation.
  const props = defineProps<{ recipient?: CirculationRecipient, circulationTitle?: string }>()
  const open = defineModel<boolean>({ default: false })
  const emit = defineEmits<{ responded: [] }>()

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const response = ref<CirculationResponse>('CIRCULATION_RESPONSE_FAVORABLE')
  const text = ref('')
  const busy = ref(false)
  const textRequired = computed(() => responseNeedsText(response.value))

  watch(open, isOpen => {
    if (isOpen) {
      response.value = 'CIRCULATION_RESPONSE_FAVORABLE'
      text.value = ''
    }
  })

  async function submit () {
    if (!props.recipient || (textRequired.value && !text.value.trim())) return
    busy.value = true
    try {
      await respondToCirculation(props.recipient.id, response.value, text.value.trim() || undefined)
      ui.notify(t('circulations.messages.responded'), 'success')
      open.value = false
      emit('responded')
    } catch (error) {
      report(error)
    } finally {
      busy.value = false
    }
  }
</script>

<template>
  <v-dialog v-model="open" max-width="560">
    <v-card>
      <v-card-title>{{ t('circulations.actions.respond') }}</v-card-title>

      <v-card-text>
        <p class="text-body-2 mb-1">{{ circulationTitle }}</p>
        <p class="text-caption text-medium-emphasis mb-3">{{ t('circulations.respondFor', { recipient: recipient?.assigneeLabel }) }}</p>

        <v-radio-group v-model="response" density="compact">
          <v-radio v-for="r in RESPONSES" :key="r" :label="enumLabel('CirculationResponse', r)" :value="r" />
        </v-radio-group>

        <v-textarea
          v-model="text"
          auto-grow
          :label="t('circulations.fields.responseText')"
          rows="3"
          :rules="textRequired ? [required(t), maxLength(t, 4000)] : [maxLength(t, 4000)]"
        />

        <p class="text-caption text-medium-emphasis">{{ t('circulations.respondHint') }}</p>
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="open = false">{{ t('actions.common.cancel') }}</v-btn>
        <v-btn color="primary" :loading="busy" variant="flat" @click="submit">{{ t('actions.common.confirm') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
