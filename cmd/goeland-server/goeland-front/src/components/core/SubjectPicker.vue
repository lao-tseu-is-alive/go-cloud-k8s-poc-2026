<script setup lang="ts">
  import type { SubjectKind, SubjectRef } from '@/api/types'
  import { computed, onBeforeUnmount, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { searchActors } from '@/api/actorClient'
  import { searchCases } from '@/api/caseClient'
  import { searchDocuments } from '@/api/documentClient'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { kindIcon } from '@/utils/subjects'

  // Searches the subjects of one kind (server-side, accent-insensitive) and binds
  // the chosen subject's id, so nobody has to paste a UUID.
  const props = defineProps<{ kind: SubjectKind, excludeId?: string, label?: string }>()
  const model = defineModel<string | undefined>()

  const { t } = useI18n()
  const { report } = useApiErrors()
  const PAGE_SIZE = 20
  const DEBOUNCE_MS = 250

  type Searcher = (query: string, signal: AbortSignal) => Promise<(SubjectRef | undefined)[]>
  const SEARCHERS: Partial<Record<SubjectKind, Searcher>> = {
    SUBJECT_KIND_ACTOR: async (query, signal) =>
      ((await searchActors({ query, onlyActive: true, pageSize: PAGE_SIZE }, signal)).actors ?? []).map(a => a.subjectRef),
    SUBJECT_KIND_CASE: async (query, signal) =>
      ((await searchCases({ query, pageSize: PAGE_SIZE }, signal)).cases ?? []).map(c => c.subjectRef),
    SUBJECT_KIND_DOCUMENT: async (query, signal) =>
      ((await searchDocuments({ query, pageSize: PAGE_SIZE }, signal)).documents ?? []).map(d => d.subjectRef),
  }

  const items = ref<SubjectRef[]>([])
  const loading = ref(false)
  const search = ref('')
  let controller: AbortController | undefined
  let timer: ReturnType<typeof setTimeout> | undefined

  const searchable = computed(() => SEARCHERS[props.kind] !== undefined)

  async function run (query: string) {
    const searcher = SEARCHERS[props.kind]
    if (!searcher) return
    controller?.abort()
    controller = new AbortController()
    loading.value = true
    try {
      const found = await searcher(query.trim(), controller.signal)
      items.value = found.filter((s): s is SubjectRef => !!s && s.id !== props.excludeId)
    } catch (error) {
      if (!(error instanceof DOMException && error.name === 'AbortError')) report(error)
    } finally {
      loading.value = false
    }
  }

  // Typing a new term searches again; picking an item also updates the search
  // text to its label, which must not trigger a search.
  watch(search, term => {
    const picked = items.value.find(s => s.id === model.value)
    if (picked && term === picked.displayLabel) return
    clearTimeout(timer)
    timer = setTimeout(() => void run(term ?? ''), DEBOUNCE_MS)
  })
  watch(() => props.kind, () => {
    model.value = undefined
    items.value = []
    void run('')
  }, { immediate: true })

  onBeforeUnmount(() => {
    clearTimeout(timer)
    controller?.abort()
  })

  function subtitle (s: SubjectRef): string {
    return [s.businessRefNamespace, s.businessRef].filter(Boolean).join(' ')
  }
</script>

<template>
  <v-autocomplete
    v-if="searchable"
    v-model="model"
    v-model:search="search"
    clearable
    :hint="t('link.pickerHint')"
    item-title="displayLabel"
    item-value="id"
    :items="items"
    :label="props.label ?? t('link.target')"
    :loading="loading"
    :no-data-text="t('link.noMatch')"
    no-filter
    persistent-hint
    :prepend-inner-icon="kindIcon(props.kind)"
  >
    <template #item="{ props: itemProps, item }">
      <v-list-item v-bind="itemProps" :subtitle="subtitle(item)" />
    </template>
  </v-autocomplete>

  <!-- Kinds without a search endpoint yet (THING, USER, ORG_UNIT): id entry. -->
  <v-text-field
    v-else
    v-model="model"
    :label="t('link.targetId')"
    placeholder="00000000-0000-0000-0000-000000000000"
  />
</template>
