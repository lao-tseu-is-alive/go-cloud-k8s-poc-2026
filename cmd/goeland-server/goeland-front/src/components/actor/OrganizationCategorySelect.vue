<script setup lang="ts">
  import type { OrganizationCategory } from '@/api/types'
  import { onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { listOrganizationCategories } from '@/api/actorClient'
  import { useApiErrors } from '@/composables/useApiErrors'

  // Bound value is the organization category *code* (sent to the API). Labels
  // come from the catalogue's own label field (seeded from real prod categories).
  const props = defineProps<{
    label?: string
    clearable?: boolean
    rules?: Array<(v: unknown) => true | string>
  }>()
  const model = defineModel<string | undefined>()

  const { t } = useI18n()
  const { report } = useApiErrors()
  const categories = ref<OrganizationCategory[]>([])
  const loading = ref(false)

  onMounted(async () => {
    loading.value = true
    try {
      categories.value = await listOrganizationCategories(true)
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
    :items="categories"
    :label="props.label ?? t('fields.actor.category')"
    :loading="loading"
    :rules="props.rules"
  />
</template>
