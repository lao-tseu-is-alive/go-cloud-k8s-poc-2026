<script setup lang="ts">
  import type { FinalDisposition } from '@/api/types'
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { createDispositionBordereau } from '@/api/retentionClient'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { maxLength, required } from '@/utils/validation'

  // Prepares a disposition bordereau of the selected due cases (GLD-032): one
  // final disposition (a case of a REVIEW policy fits either) and a reason.
  const props = defineProps<{ caseIds: string[] }>()
  const open = defineModel<boolean>({ default: false })
  const emit = defineEmits<{ created: [id: string] }>()
  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const { report } = useApiErrors()

  const DISPOSITIONS: FinalDisposition[] = ['FINAL_DISPOSITION_ARCHIVE', 'FINAL_DISPOSITION_DESTROY']
  const disposition = ref<FinalDisposition>('FINAL_DISPOSITION_ARCHIVE')
  const reason = ref('')
  const busy = ref(false)
  const form = ref()
  const items = computed(() => DISPOSITIONS.map(d => ({ value: d, title: enumLabel('FinalDisposition', d) })))

  watch(open, value => {
    if (value) reason.value = ''
  })

  async function submit () {
    const validation = await form.value?.validate()
    if (validation && !validation.valid) return
    busy.value = true
    try {
      const b = await createDispositionBordereau(disposition.value, reason.value.trim(), props.caseIds)
      open.value = false
      if (b) emit('created', b.id)
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
      <v-card-title>{{ t('retention.bordereau.create') }}</v-card-title>

      <v-card-text>
        <p class="text-caption text-medium-emphasis mb-3">{{ t('retention.bordereau.createHint', { n: caseIds.length }) }}</p>

        <v-form ref="form" @submit.prevent="submit">
          <v-select
            v-model="disposition"
            item-title="title"
            item-value="value"
            :items="items"
            :label="t('retention.page.disposition')"
          />

          <v-textarea
            v-model="reason"
            auto-grow
            counter="2000"
            :label="t('retention.bordereau.reason')"
            rows="2"
            :rules="[required(t), maxLength(t, 2000)]"
          />
        </v-form>
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="open = false">{{ t('actions.common.cancel') }}</v-btn>
        <v-btn color="primary" :loading="busy" variant="flat" @click="submit">{{ t('retention.bordereau.prepare') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
