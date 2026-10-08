<script setup lang="ts">
  import type { CaseType } from '@/api/types'
  import { onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { listCaseTypes } from '@/api/caseClient'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { foldedFilter } from '@/utils/textSearch'

  // A searchable picker (GLD-057): the catalogue comes ordered by label and typing filters it.
  // Bound value is the case_type *code* (sent to the API). Labels come from the
  // catalogue's own label field.
  const props = defineProps<{
    label?: string
    clearable?: boolean
    rules?: Array<(v: unknown) => true | string>
  }>()
  const model = defineModel<string | undefined>()

  const { t } = useI18n()
  const { report } = useApiErrors()
  const types = ref<CaseType[]>([])
  const loading = ref(false)

  onMounted(async () => {
    loading.value = true
    try {
      types.value = await listCaseTypes(true)
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  })
</script>

<template>
  <v-autocomplete
    v-model="model"
    auto-select-first
    :clearable="props.clearable"
    :custom-filter="foldedFilter"
    item-title="label"
    item-value="code"
    :items="types"
    :label="props.label ?? t('fields.case.case_type')"
    :loading="loading"
    :rules="props.rules"
  />
</template>
