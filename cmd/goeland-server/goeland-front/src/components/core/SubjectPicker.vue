<script setup lang="ts">
  import type { SubjectKind, SubjectRef } from '@/api/types'
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { searchActors } from '@/api/actorClient'
  import { searchCases } from '@/api/caseClient'
  import { searchUsers } from '@/api/coreClient'
  import { searchDocuments } from '@/api/documentClient'
  import { searchOrgUnits } from '@/api/orgUnitClient'
  import { searchThings } from '@/api/thingClient'
  import { useDebouncedSearch } from '@/composables/useDebouncedSearch'
  import { kindIcon } from '@/utils/subjects'

  // Searches the subjects of one kind (server-side, accent-insensitive) and binds
  // the chosen subject's id, so nobody has to paste a UUID.
  const props = defineProps<{ kind: SubjectKind, excludeId?: string, label?: string }>()
  const model = defineModel<string | undefined>()
  // picked carries the whole chosen subject, for callers that show its label.
  const emit = defineEmits<{ picked: [subject: SubjectRef] }>()

  const { t } = useI18n()
  const PAGE_SIZE = 20

  type Searcher = (query: string, signal: AbortSignal) => Promise<(SubjectRef | undefined)[]>
  const SEARCHERS: Partial<Record<SubjectKind, Searcher>> = {
    SUBJECT_KIND_ACTOR: async (query, signal) =>
      ((await searchActors({ query, onlyActive: true, pageSize: PAGE_SIZE }, signal)).actors ?? []).map(a => a.subjectRef),
    SUBJECT_KIND_CASE: async (query, signal) =>
      ((await searchCases({ query, pageSize: PAGE_SIZE }, signal)).cases ?? []).map(c => c.subjectRef),
    SUBJECT_KIND_THING: async (query, signal) =>
      ((await searchThings({ query, pageSize: PAGE_SIZE }, signal)).things ?? []).map(th => th.subjectRef),
    SUBJECT_KIND_DOCUMENT: async (query, signal) =>
      ((await searchDocuments({ query, pageSize: PAGE_SIZE }, signal)).documents ?? []).map(d => d.subjectRef),
    // An internal user reads as its USER subject (its id is the subject id).
    SUBJECT_KIND_USER: async (query, signal) =>
      (await searchUsers(query, PAGE_SIZE, signal)).map(u => u.subjectId
        ? { id: u.subjectId, kind: 'SUBJECT_KIND_USER', displayLabel: u.displayName || u.email || u.id }
        : undefined),
    // Dissolved units take no new relationship: only live ones are offered.
    SUBJECT_KIND_ORG_UNIT: async (query, signal) =>
      ((await searchOrgUnits({ query, pageSize: PAGE_SIZE }, signal)).units ?? []).map(u => u.subjectRef),
  }

  const search = ref('')
  const searchable = computed(() => SEARCHERS[props.kind] !== undefined)

  const { results: items, loading, run, schedule } = useDebouncedSearch<SubjectRef[]>(async (query, signal) => {
    const searcher = SEARCHERS[props.kind]
    if (!searcher) return []
    const found = await searcher(query, signal)
    return found.filter((s): s is SubjectRef => !!s && s.id !== props.excludeId)
  }, [])

  // Typing a new term searches again; picking an item also updates the search
  // text to its label, which must not trigger a search.
  watch(search, term => {
    const picked = items.value.find(s => s.id === model.value)
    if (picked && term === picked.displayLabel) return
    schedule(term ?? '')
  })
  watch(model, id => {
    const picked = items.value.find(s => s.id === id)
    if (picked) emit('picked', picked)
  })
  watch(() => props.kind, () => {
    model.value = undefined
    items.value = []
    void run('')
  }, { immediate: true })

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

  <!-- Kinds without a search endpoint: id entry. -->
  <v-text-field
    v-else
    v-model="model"
    :label="t('link.targetId')"
    placeholder="00000000-0000-0000-0000-000000000000"
  />
</template>
