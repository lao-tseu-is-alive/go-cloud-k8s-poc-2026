<script setup lang="ts">
  import type { SubjectKind, SubjectSearchGroup } from '@/api/types'
  import { computed, reactive, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { searchSubjects } from '@/api/coreClient'
  import ActorSearchList from '@/components/actor/ActorSearchList.vue'
  import CaseSearchList from '@/components/case/CaseSearchList.vue'
  import DocumentSearchList from '@/components/document/DocumentSearchList.vue'
  import ThingSearchList from '@/components/thing/ThingSearchList.vue'
  import { useDebouncedSearch } from '@/composables/useDebouncedSearch'
  import { formatTotal } from '@/utils/formatters'
  import { searchable } from '@/utils/globalSearch'
  import { kindIcon } from '@/utils/subjects'

  // Every result of the global search (GLD-064, GLD-074): one tab per kind
  // with its count, the active tab showing the kind's list with all its
  // filters, the query already set. The tab and the filters live in the URL.
  const TABS = ['SUBJECT_KIND_CASE', 'SUBJECT_KIND_ACTOR', 'SUBJECT_KIND_DOCUMENT', 'SUBJECT_KIND_THING'] as const
  type Tab = typeof TABS[number]
  const isTab = (v: unknown): v is Tab => TABS.includes(v as Tab)

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()

  const query = computed(() => typeof route.query.q === 'string' ? route.query.q.trim() : '')
  // Only the counts are needed: the lists load their own rows.
  const { results: groups, loading, run } = useDebouncedSearch<SubjectSearchGroup[]>(
    (q, signal) => searchSubjects({ query: q, limitPerKind: 1 }, signal), [])
  // Once a list has loaded, its own total replaces the global count: it also
  // matches descriptions and follows the filters.
  const listTotals = reactive<Partial<Record<Tab, { size: number, capped: boolean }>>>({})

  watch(query, q => {
    for (const k of TABS) delete listTotals[k]
    if (searchable(q)) {
      void run(q)
    } else {
      groups.value = []
    }
  }, { immediate: true })

  function groupOf (kind: Tab): SubjectSearchGroup | undefined {
    return groups.value.find(g => g.kind === kind)
  }

  function count (kind: Tab): string {
    const own = listTotals[kind]
    if (own) return formatTotal(own.size, own.capped)
    const group = groupOf(kind)
    return formatTotal(group?.totalSize ?? 0, group?.totalSizeCapped)
  }

  // The tab in the URL, otherwise the first kind with results.
  const tab = computed<Tab>({
    get: () => {
      if (isTab(route.query.tab)) return route.query.tab
      return TABS.find(k => (groupOf(k)?.totalSize ?? 0) > 0) ?? TABS[0]
    },
    // Another kind starts from the query alone: the filters of a list do not apply to the next.
    set: kind => void router.replace({ query: { q: query.value, tab: kind } }),
  })

  function onTotal (kind: SubjectKind, size: number, capped: boolean) {
    if (isTab(kind)) listTotals[kind] = { size, capped }
  }
</script>

<template>
  <v-container fluid>
    <h1 class="text-h5 mb-4">{{ t('globalSearch.resultsTitle', { query }) }}</h1>

    <v-alert v-if="!searchable(query)" type="info" variant="tonal">{{ t('globalSearch.minChars') }}</v-alert>

    <template v-else>
      <v-tabs v-model="tab" class="mb-4" color="primary" show-arrows>
        <v-tab v-for="kind in TABS" :key="kind" :prepend-icon="kindIcon(kind)" :value="kind">
          {{ t(`globalSearch.kinds.${kind}`) }}
          <v-chip class="ms-2" label size="x-small">{{ count(kind) }}</v-chip>
        </v-tab>
      </v-tabs>

      <v-progress-linear v-if="loading" color="primary" indeterminate />

      <!-- Mounted once the counts are known, so the list loads once, in the right tab. -->
      <template v-else>
        <CaseSearchList v-if="tab === 'SUBJECT_KIND_CASE'" @total="(size, capped) => onTotal(tab, size, capped)" />
        <ActorSearchList v-else-if="tab === 'SUBJECT_KIND_ACTOR'" @total="(size, capped) => onTotal(tab, size, capped)" />
        <DocumentSearchList v-else-if="tab === 'SUBJECT_KIND_DOCUMENT'" @total="(size, capped) => onTotal(tab, size, capped)" />
        <ThingSearchList v-else @total="(size, capped) => onTotal(tab, size, capped)" />
      </template>
    </template>
  </v-container>
</template>
