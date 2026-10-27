<script setup lang="ts">
  import type { SubjectSearchGroup } from '@/api/types'
  import { computed, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute } from 'vue-router'
  import { searchSubjects } from '@/api/coreClient'
  import { useDebouncedSearch } from '@/composables/useDebouncedSearch'
  import { formatTotal } from '@/utils/formatters'
  import { filteredListRoute, groupTitle, searchable, subjectReference } from '@/utils/globalSearch'
  import { kindIcon, subjectRoute } from '@/utils/subjects'

  // Every result of the global search (GLD-064): per kind the 25 best subjects,
  // and a link to the kind's list filtered by the query for the rest.
  const PER_KIND = 25
  const { t } = useI18n()
  const route = useRoute()

  const query = computed(() => typeof route.query.q === 'string' ? route.query.q.trim() : '')
  const { results: groups, loading, run } = useDebouncedSearch<SubjectSearchGroup[]>(
    (q, signal) => searchSubjects({ query: q, limitPerKind: PER_KIND }, signal), [])
  const found = computed(() => groups.value.filter(g => (g.totalSize ?? 0) > 0))

  watch(query, q => {
    if (searchable(q)) {
      void run(q)
    } else {
      groups.value = []
    }
  }, { immediate: true })
</script>

<template>
  <v-container fluid>
    <h1 class="text-h5 mb-4">{{ t('globalSearch.resultsTitle', { query }) }}</h1>

    <v-progress-linear v-if="loading" class="mb-4" color="primary" indeterminate />

    <v-alert v-if="!searchable(query)" type="info" variant="tonal">{{ t('globalSearch.minChars') }}</v-alert>

    <v-alert v-else-if="!loading && found.length === 0" type="info" variant="tonal">
      {{ t('globalSearch.noResult', { query }) }}
    </v-alert>

    <v-row>
      <v-col v-for="group in found" :key="group.kind" cols="12" lg="6">
        <v-card variant="outlined">
          <v-card-title class="d-flex align-center ga-2">
            <v-icon :icon="kindIcon(group.kind)" size="small" />
            {{ groupTitle(t, group) }}
          </v-card-title>

          <v-list density="compact">
            <v-list-item
              v-for="s in group.subjects ?? []"
              :key="s.id"
              :subtitle="subjectReference(s) || undefined"
              :title="s.displayLabel"
              :to="subjectRoute(s)"
            />
          </v-list>

          <v-card-actions v-if="filteredListRoute(group.kind, query) && (group.totalSize ?? 0) > (group.subjects?.length ?? 0)">
            <v-btn append-icon="mdi-arrow-right" color="primary" :to="filteredListRoute(group.kind, query)" variant="text">
              {{ t('globalSearch.seeAll', { count: formatTotal(group.totalSize, group.totalSizeCapped), list: t(`globalSearch.kinds.${group.kind}`) }) }}
            </v-btn>
          </v-card-actions>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>
