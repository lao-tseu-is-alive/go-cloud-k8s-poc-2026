<script setup lang="ts">
  import type { SecurityGroup } from '@/api/types'
  import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
  import { listGroups } from '@/api/accessClient'
  import { useApiErrors } from '@/composables/useApiErrors'

  // Finds a live security group by name (debounced) and binds its subject id.
  const model = defineModel<string | undefined>()
  const props = defineProps<{ label: string }>()
  const { report } = useApiErrors()
  const DEBOUNCE_MS = 250

  const groups = ref<SecurityGroup[]>([])
  const search = ref('')
  const loading = ref(false)
  let timer: ReturnType<typeof setTimeout> | undefined

  async function find (query: string) {
    loading.value = true
    try {
      groups.value = await listGroups(query.trim())
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  watch(search, term => {
    clearTimeout(timer)
    timer = setTimeout(() => void find(term ?? ''), DEBOUNCE_MS)
  })
  onMounted(() => void find(''))
  onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <v-autocomplete
    v-model="model"
    v-model:search="search"
    clearable
    :item-title="(g: SecurityGroup) => g.name"
    :item-value="(g: SecurityGroup) => g.subjectRef?.id"
    :items="groups"
    :label="props.label"
    :loading="loading"
    no-filter
    prepend-inner-icon="mdi-account-group-outline"
  />
</template>
