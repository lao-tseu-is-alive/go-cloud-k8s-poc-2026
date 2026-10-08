<script setup lang="ts">
  import type { RelationshipType, SubjectKind } from '@/api/types'
  import { onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { listRelationshipTypes } from '@/api/coreClient'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { foldedFilter } from '@/utils/textSearch'

  // Loads the relationship-type catalogue (ordered by label) and lets the user search and
  // pick one (GLD-057). The bound value is always the *code* (sent to the API); the label
  // is display-only.
  const props = defineProps<{
    sourceKind?: SubjectKind
    targetKind?: SubjectKind
    label?: string
  }>()
  const model = defineModel<string | undefined>()
  // The selected type itself, so callers can follow its target kind.
  const emit = defineEmits<{ selected: [type: RelationshipType | undefined] }>()

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
  <v-autocomplete
    v-model="model"
    auto-select-first
    :custom-filter="foldedFilter"
    item-title="label"
    item-value="code"
    :items="types"
    :label="props.label ?? t('link.chooseType')"
    :loading="loading"
    @update:model-value="code => emit('selected', types.find(rt => rt.code === code))"
  />
</template>
