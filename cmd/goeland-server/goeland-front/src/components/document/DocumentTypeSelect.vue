<script setup lang="ts">
  import type { DocumentType } from '@/api/types'
  import { onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { listDocumentTypes } from '@/api/documentClient'
  import { useApiErrors } from '@/composables/useApiErrors'

  // Bound value is the document_type *code* (sent to the API). Labels come from
  // the catalogue's own label field.
  const props = defineProps<{
    label?: string
    clearable?: boolean
    rules?: Array<(v: unknown) => true | string>
  }>()
  const model = defineModel<string | undefined>()

  const { t } = useI18n()
  const { report } = useApiErrors()
  const types = ref<DocumentType[]>([])
  const loading = ref(false)

  onMounted(async () => {
    loading.value = true
    try {
      types.value = await listDocumentTypes(true)
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  })
</script>

<template>
  <v-select
    v-model="model"
    :clearable="props.clearable"
    item-title="label"
    item-value="code"
    :items="types"
    :label="props.label ?? t('fields.document.document_type')"
    :loading="loading"
    :rules="props.rules"
  />
</template>
