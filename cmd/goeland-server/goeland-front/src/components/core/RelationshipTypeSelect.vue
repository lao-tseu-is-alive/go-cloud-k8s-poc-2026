<script setup lang="ts">
  import type { RelationshipType, SubjectKind } from '@/api/types'
  import { onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { listRelationshipTypes } from '@/api/coreClient'
  import { useApiErrors } from '@/composables/useApiErrors'

  // Loads the relationship-type catalogue and lets the user pick one. The bound
  // value is always the *code* (sent to the API); the label is display-only.
  const props = defineProps<{
    sourceKind?: SubjectKind
    targetKind?: SubjectKind
    label?: string
  }>()
  const model = defineModel<string | undefined>()

  const { t } = useI18n()
  const { report } = useApiErrors()
  const types = ref<RelationshipType[]>([])
  const loading = ref(false)

  onMounted(async () => {
    loading.value = true
    try {
      types.value = await listRelationshipTypes({
        onlyActive: true,
        sourceKind: props.sourceKind,
        targetKind: props.targetKind,
      })
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
    item-title="label"
    item-value="code"
    :items="types"
    :label="props.label ?? t('link.chooseType')"
    :loading="loading"
  />
</template>
