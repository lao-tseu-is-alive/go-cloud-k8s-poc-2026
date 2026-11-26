<script setup lang="ts">
  import type { DocumentVersion } from '@/api/types'
  import { computed, onBeforeUnmount, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { ApiError } from '@/api/client'
  import { fetchDocumentContent } from '@/api/documentClient'
  import { downloadable, PREVIEW_TEXT_CHARS, previewKind, previewLoadsByItself } from '@/utils/documents'
  import { formatBytes } from '@/utils/formatters'

  // Shows the content of one document version inline (GLD-087): an image, a PDF
  // in the browser's viewer or a text. The bytes come from the governed download
  // with the bearer token and are shown from an object URL (the CSP allows blob:
  // images and frames). A PDF is framed from a blob typed application/pdf here,
  // whatever the server said, so no other content is ever rendered as a page of
  // this origin; a text is shown as text, never as markup. A confidential
  // document waits for the viewer to ask, because every read of its content is
  // recorded (DOWNLOAD).
  const props = defineProps<{
    documentId: string
    version: DocumentVersion
    title?: string
    /** Every content read is audited (confidentiality level 2 or more). */
    audited?: boolean
  }>()

  const { t } = useI18n()
  const content = computed(() => props.version.content)
  const kind = computed(() => downloadable(content.value?.storageRef) ? previewKind(content.value?.mimeType) : null)
  const state = ref<'idle' | 'loading' | 'ready' | 'missing' | 'denied' | 'failed'>('idle')
  const url = ref('')
  const text = ref('')
  const truncated = ref(false)
  // Answers of an older request (another version chosen meanwhile) are dropped.
  let request = 0

  function release () {
    if (url.value) {
      URL.revokeObjectURL(url.value)
    }
    url.value = ''
    text.value = ''
    truncated.value = false
  }

  function failure (error: unknown): typeof state.value {
    if (error instanceof ApiError && error.status === 404) return 'missing'
    if (error instanceof ApiError && error.status === 403) return 'denied'
    return 'failed'
  }

  async function show (blob: Blob) {
    if (kind.value === 'text') {
      const all = await blob.text()
      truncated.value = all.length > PREVIEW_TEXT_CHARS
      text.value = truncated.value ? all.slice(0, PREVIEW_TEXT_CHARS) : all
      return
    }
    const typed = kind.value === 'pdf' ? new Blob([blob], { type: 'application/pdf' }) : blob
    url.value = URL.createObjectURL(typed)
  }

  async function load () {
    const mine = ++request
    release()
    state.value = 'loading'
    try {
      const blob = await fetchDocumentContent(props.documentId, props.version.id)
      if (mine !== request) return
      await show(blob)
      state.value = 'ready'
    } catch (error) {
      if (mine === request) state.value = failure(error)
    }
  }

  watch(() => [props.documentId, props.version.id, kind.value], () => {
    request++
    release()
    state.value = 'idle'
    if (kind.value && previewLoadsByItself(kind.value, content.value?.fileSizeBytes, props.audited)) {
      void load()
    }
  }, { immediate: true })

  onBeforeUnmount(() => {
    request++
    release()
  })
</script>

<template>
  <div class="document-preview">
    <div v-if="!kind" class="text-medium-emphasis text-body-2">
      {{ content ? t('documentPreview.unsupported') : t('documentPreview.noContent') }}
    </div>

    <template v-else>
      <div v-if="state === 'idle'" class="document-preview__ask">
        <p class="text-body-2 text-medium-emphasis mb-3">
          {{ audited ? t('documentPreview.askAudited') : t('documentPreview.askLarge', { size: formatBytes(content?.fileSizeBytes) }) }}
        </p>

        <v-btn prepend-icon="mdi-eye" variant="tonal" @click="load">{{ t('documentPreview.show') }}</v-btn>
      </div>

      <v-progress-linear v-else-if="state === 'loading'" color="primary" indeterminate />

      <v-alert
        v-else-if="state !== 'ready'"
        density="compact"
        type="warning"
        variant="tonal"
      >{{ t(`documentPreview.${state}`) }}</v-alert>

      <img
        v-else-if="kind === 'image'"
        :alt="title || t('documentPreview.imageAlt')"
        class="document-preview__image"
        :src="url"
      >

      <iframe
        v-else-if="kind === 'pdf'"
        class="document-preview__frame"
        :src="url"
        :title="title || t('documentPreview.pdfTitle')"
      />

      <template v-else>
        <pre class="document-preview__text">{{ text }}</pre>
        <p v-if="truncated" class="text-caption text-medium-emphasis mt-2">{{ t('documentPreview.truncated') }}</p>
      </template>
    </template>
  </div>
</template>

<style scoped>
  .document-preview__image {
    display: block;
    max-width: 100%;
    max-height: 70vh;
    margin: 0 auto;
    object-fit: contain;
    border-radius: 4px;
  }

  .document-preview__frame {
    display: block;
    width: 100%;
    height: 75vh;
    border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
    border-radius: 4px;
  }

  .document-preview__text {
    max-height: 70vh;
    overflow: auto;
    padding: 12px;
    font-size: 0.85rem;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    background: rgba(var(--v-theme-on-surface), 0.04);
    border-radius: 4px;
  }
</style>
