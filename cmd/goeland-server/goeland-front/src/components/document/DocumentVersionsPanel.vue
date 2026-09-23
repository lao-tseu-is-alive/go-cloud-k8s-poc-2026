<script setup lang="ts">
  import type { DocumentVersion, UploadResult } from '@/api/types'
  import { onMounted, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { addDocumentVersion, listDocumentVersions } from '@/api/documentClient'
  import DocumentUploadField from '@/components/document/DocumentUploadField.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useUiStore } from '@/stores/ui'
  import { formatBytes, formatDateTime, shortHash } from '@/utils/formatters'

  // Lists the append-only versions of a document (newest first) and, when the
  // document is editable, adds a new current version from an uploaded file.
  const props = defineProps<{ documentId: string, currentVersionId?: string, canAdd?: boolean }>()
  const emit = defineEmits<{ changed: [] }>()

  const { t } = useI18n()
  const { report } = useApiErrors()
  const ui = useUiStore()
  const versions = ref<DocumentVersion[]>([])
  const loading = ref(false)
  const adding = ref(false)
  const busy = ref(false)
  const upload = ref<UploadResult | null>(null)
  const reason = ref('')

  async function load () {
    loading.value = true
    try {
      versions.value = await listDocumentVersions(props.documentId)
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  async function add () {
    busy.value = true
    try {
      await addDocumentVersion(props.documentId, {
        contentBlobId: upload.value?.contentBlobId,
        reason: reason.value.trim() || undefined,
      })
      ui.notify(t('messages.document.versionAdded'), 'success')
      adding.value = false
      upload.value = null
      reason.value = ''
      emit('changed')
    } catch (error) {
      report(error)
    } finally {
      busy.value = false
    }
  }

  onMounted(load)
  // The parent reloads the document after a change; follow its current version.
  watch(() => props.currentVersionId, load)
</script>

<template>
  <div>
    <v-progress-linear v-if="loading" class="mb-2" color="primary" indeterminate />

    <v-table v-if="versions.length > 0" density="compact">
      <thead>
        <tr>
          <th>{{ t('versions.number') }}</th>
          <th>{{ t('versions.content') }}</th>
          <th />
          <th>{{ t('fields.common.created_at') }}</th>
        </tr>
      </thead>

      <tbody>
        <tr v-for="v in versions" :key="v.id">
          <td>
            {{ v.versionNo }}
            <v-chip v-if="v.id === currentVersionId" class="ml-1" color="primary" size="x-small">{{ t('versions.current') }}</v-chip>
          </td>

          <td v-if="v.content" class="text-caption">
            {{ v.content.mimeType || '—' }} · {{ formatBytes(v.content.fileSizeBytes) }} · {{ shortHash(v.content.sha256) }}
          </td>

          <td v-else class="text-caption text-medium-emphasis">{{ t('versions.noContent') }}</td>

          <td>
            <v-chip v-if="v.isRecord" color="primary" size="x-small">{{ t('states.record') }}</v-chip>
            <v-chip v-else-if="v.isFinal" color="success" size="x-small">{{ t('states.final') }}</v-chip>
          </td>

          <td class="text-caption">{{ formatDateTime(v.createdAt) }}</td>
        </tr>
      </tbody>
    </v-table>

    <div v-else-if="!loading" class="text-medium-emphasis">{{ t('versions.empty') }}</div>

    <template v-if="canAdd">
      <v-btn
        v-if="!adding"
        class="mt-3"
        prepend-icon="mdi-file-plus"
        variant="tonal"
        @click="adding = true"
      >
        {{ t('versions.add') }}
      </v-btn>

      <div v-else class="mt-3">
        <div class="text-caption text-medium-emphasis mb-2">{{ t('versions.addHint') }}</div>
        <DocumentUploadField @uploaded="upload = $event" />
        <v-text-field v-model="reason" density="compact" :label="t('finalize.reason')" />

        <div class="d-flex justify-end ga-2">
          <v-btn variant="text" @click="adding = false">{{ t('actions.common.cancel') }}</v-btn>
          <v-btn color="primary" :loading="busy" variant="flat" @click="add">{{ t('versions.add') }}</v-btn>
        </div>
      </div>
    </template>
  </div>
</template>
