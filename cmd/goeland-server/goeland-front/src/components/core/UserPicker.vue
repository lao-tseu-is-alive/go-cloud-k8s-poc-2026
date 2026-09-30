<script setup lang="ts">
  import type { User } from '@/api/types'
  import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
  import { searchUsers } from '@/api/coreClient'
  import { useApiErrors } from '@/composables/useApiErrors'

  // Finds an internal user by name or e-mail (debounced server search) and
  // binds its operator id; emits the picked user so callers can show its name.
  const model = defineModel<string | undefined>()
  const props = defineProps<{ label: string }>()
  const emit = defineEmits<{ picked: [user: User | undefined] }>()
  const { report } = useApiErrors()
  const DEBOUNCE_MS = 250

  const users = ref<User[]>([])
  const search = ref('')
  const loading = ref(false)
  let timer: ReturnType<typeof setTimeout> | undefined
  let controller: AbortController | undefined

  async function find (query: string) {
    controller?.abort()
    controller = new AbortController()
    loading.value = true
    try {
      users.value = await searchUsers(query.trim(), 20, controller.signal)
    } catch (error) {
      if (!(error instanceof DOMException && error.name === 'AbortError')) report(error)
    } finally {
      loading.value = false
    }
  }

  function pick (userId?: string) {
    model.value = userId || undefined
    emit('picked', users.value.find(u => u.id === userId))
  }

  watch(search, term => {
    clearTimeout(timer)
    timer = setTimeout(() => void find(term ?? ''), DEBOUNCE_MS)
  })
  onMounted(() => void find(''))
  onBeforeUnmount(() => {
    clearTimeout(timer)
    controller?.abort()
  })
</script>

<template>
  <v-autocomplete
    v-model:search="search"
    clearable
    :item-title="(u: User) => u.displayName || u.email || u.id"
    item-value="id"
    :items="users"
    :label="props.label"
    :loading="loading"
    :model-value="model"
    no-filter
    prepend-inner-icon="mdi-account-search-outline"
    @update:model-value="pick"
  />
</template>
