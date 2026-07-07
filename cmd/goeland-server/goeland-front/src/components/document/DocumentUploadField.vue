<script setup lang="ts">
  import type { UploadResult } from '@/api/types'
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { uploadDocumentFile } from '@/api/documentClient'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { formatBytes, shortHash } from '@/utils/formatters'

  // Drives the out-of-proto multipart upload endpoint. On success it emits the
  // server-computed storage_ref + integrity metadata for the create form to send
  // to CreateDocument.
  const emit = defineEmits<{ uploaded: [result: UploadResult] }>()

  const { t } = useI18n()
  const { report } = useApiErrors()
  const file = ref<File | File[] | null>(null)
  const uploading = ref(false)
  const result = ref<UploadResult | null>(null)

  async function onChange () {
    const picked = Array.isArray(file.value) ? file.value[0] : file.value
    if (!picked) return
    uploading.value = true
    result.value = null
    try {
      const res = await uploadDocumentFile(picked)
      result.value = res
      emit('uploaded', res)
    } catch (error) {
      report(error)
      file.value = null
    } finally {
      uploading.value = false
    }
  }
</script>

<template>
  <div>
    <v-file-input
      v-model="file"
      :disabled="uploading"
      :label="t('upload.chooseFile')"
      prepend-icon="mdi-paperclip"
      show-size
      @update:model-value="onChange"
    />

    <v-progress-linear v-if="uploading" class="mb-2" color="primary" indeterminate />

    <v-alert
      v-if="result"
      class="mt-1"
      density="compact"
      type="success"
      variant="tonal"
    >
      <div class="text-body-2">{{ t('upload.uploaded') }}: {{ result.filename }}</div>

      <div class="text-caption">
        {{ t('fields.document.mime_type') }}: {{ result.mimeType }} ·
        {{ t('fields.document.file_size_bytes') }}: {{ formatBytes(result.fileSizeBytes) }} ·
        {{ t('fields.document.sha256') }}: {{ shortHash(result.sha256) }}
      </div>

      <div class="text-caption text-medium-emphasis">{{ t('upload.computed') }}</div>
    </v-alert>
  </div>
</template>
