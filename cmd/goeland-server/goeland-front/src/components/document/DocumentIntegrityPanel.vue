<script setup lang="ts">
  import type { GoDocument } from '@/api/types'
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { downloadDocumentBlob, verifyDocumentIntegrity } from '@/api/documentClient'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useUiStore } from '@/stores/ui'
  import { formatDateTime } from '@/utils/formatters'

  const props = defineProps<{ document: GoDocument }>()
  const emit = defineEmits<{ verified: [] }>()

  const { t } = useI18n()
  const { report } = useApiErrors()
  const ui = useUiStore()
  const verifying = ref(false)
  const expected = ref('')

  const documentId = () => props.document.subjectRef?.id ?? ''

  const downloading = ref(false)

  async function download () {
    const ref = props.document.storageRef
    if (!ref) return
    downloading.value = true
    try {
      await downloadDocumentBlob(ref, props.document.title || 'document')
    } catch (error) {
      report(error)
    } finally {
      downloading.value = false
    }
  }

  async function verify () {
    verifying.value = true
    try {
      const res = await verifyDocumentIntegrity(documentId(), expected.value.trim() || undefined)
      if (res.verified) ui.notify(t('messages.document.integrityVerified'), 'success')
      else ui.notify(t('messages.document.integrityFailed'), 'warning')
      emit('verified')
    } catch (error) {
      report(error)
    } finally {
      verifying.value = false
    }
  }
</script>

<template>
  <div>
    <v-table density="compact">
      <tbody>
        <tr>
          <td class="text-medium-emphasis" style="width: 40%">{{ t('fields.document.sha256') }}</td>
          <td class="text-mono">{{ document.sha256 || '—' }}</td>
        </tr>

        <tr>
          <td class="text-medium-emphasis">{{ t('fields.document.sha256_verified_at') }}</td>

          <td>
            <v-chip v-if="document.sha256VerifiedAt" color="success" size="small">
              {{ t('integrity.verified') }} — {{ formatDateTime(document.sha256VerifiedAt) }}
            </v-chip>

            <v-chip v-else color="default" size="small">{{ t('integrity.notVerified') }}</v-chip>
          </td>
        </tr>

        <tr>
          <td class="text-medium-emphasis">{{ t('fields.document.storage_ref') }}</td>
          <td class="text-mono">{{ document.storageRef || '—' }}</td>
        </tr>
      </tbody>
    </v-table>

    <div class="d-flex flex-wrap align-center ga-2 mt-3">
      <v-text-field
        v-model="expected"
        density="compact"
        hide-details
        :label="t('integrity.expected')"
        style="max-width: 420px"
      />

      <v-btn
        color="primary"
        :loading="verifying"
        prepend-icon="mdi-shield-check"
        variant="tonal"
        @click="verify"
      >
        {{ t('integrity.check') }}
      </v-btn>

      <v-btn
        v-if="document.storageRef && document.storageRef.startsWith('internal://')"
        :loading="downloading"
        prepend-icon="mdi-download"
        variant="text"
        @click="download"
      >
        {{ t('actions.document.download') }}
      </v-btn>
    </div>
  </div>
</template>

<style scoped>
.text-mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.8rem;
  word-break: break-all;
}
</style>
