<script setup lang="ts">
  import type { CaseStatus, GoCase, SearchCasesParams } from '@/api/types'
  import { computed, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { searchCases } from '@/api/caseClient'
  import { CASE_STATUSES } from '@/components/case/caseForm'
  import CaseStatusChip from '@/components/case/CaseStatusChip.vue'
  import CaseTypeSelect from '@/components/case/CaseTypeSelect.vue'
  import DateRangeFields from '@/components/core/DateRangeFields.vue'
  import PagedListFooter from '@/components/core/PagedListFooter.vue'
  import RecordFilterFields from '@/components/core/RecordFilterFields.vue'
  import SearchFilterCard from '@/components/core/SearchFilterCard.vue'
  import SortableHeader from '@/components/core/SortableHeader.vue'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { usePagedSearch } from '@/composables/usePagedSearch'
  import { formatDateTime } from '@/utils/formatters'
  import { activeCount, emptyRecordFilters, RECORD_FILTER_KEYS } from '@/utils/routeFilters'

  // The case list with its filters (GLD-074), on the case page and in a tab
  // of the search results; the filters live in the URL.
  const emit = defineEmits<{ total: [size: number, capped: boolean] }>()
  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const router = useRouter()

  type CaseFilters = Pick<SearchCasesParams, 'query' | 'caseTypeCode' | 'status' | 'openedFrom' | 'openedTo' | 'createdFrom' | 'createdTo' | 'createdBy' | 'includeDeleted'>
  const MORE = ['openedFrom', 'openedTo', ...RECORD_FILTER_KEYS, 'includeDeleted'] as const
  function emptyFilters (): CaseFilters {
    return { query: '', caseTypeCode: undefined, status: undefined, openedFrom: undefined, openedTo: undefined, ...emptyRecordFilters(), includeDeleted: false }
  }

  const { filters, sort, items: cases, nextPageToken, totalSize, totalCapped, loading, load, search, reset } = usePagedSearch(
    async (params: CaseFilters) => {
      const res = await searchCases(params)
      return { ...res, items: res.cases ?? [] }
    }, emptyFilters)
  const statusItems = computed(() => CASE_STATUSES.map((s: CaseStatus) => ({ value: s, title: enumLabel('CaseStatus', s) })))
  // Each finished search reports its total (the tab count of the results page).
  watch(loading, busy => busy || emit('total', totalSize.value, totalCapped.value))

  function openCase (c: GoCase) {
    const id = c.subjectRef?.id
    if (id) router.push(`/cases/${id}`)
  }
</script>

<template>
  <SearchFilterCard :more-count="activeCount(filters, MORE)" @reset="reset" @search="search">
    <v-col cols="12" md="6">
      <v-text-field
        v-model="filters.query"
        clearable
        density="compact"
        hide-details
        :label="t('fields.search.query')"
        prepend-inner-icon="mdi-magnify"
        :title="t('pages.cases.list.queryHint')"
        @keydown.enter="search"
      />
    </v-col>

    <v-col cols="12" md="3">
      <CaseTypeSelect v-model="filters.caseTypeCode" clearable density="compact" hide-details />
    </v-col>

    <v-col cols="12" md="3">
      <v-select
        v-model="filters.status"
        clearable
        density="compact"
        hide-details
        :items="statusItems"
        :label="t('fields.case.status')"
      />
    </v-col>

    <template #more>
      <DateRangeFields v-model:from="filters.openedFrom" v-model:to="filters.openedTo" :label="t('filters.opened')" @search="search" />
      <RecordFilterFields v-model="filters" @search="search" />

      <v-col cols="12" md="2">
        <v-checkbox v-model="filters.includeDeleted" density="compact" hide-details :label="t('fields.document.include_deleted')" />
      </v-col>
    </template>
  </SearchFilterCard>

  <v-card>
    <v-table hover>
      <thead>
        <tr>
          <SortableHeader v-model:sort="sort" field="business_ref" :label="t('fields.subject.business_ref')" />
          <SortableHeader v-model:sort="sort" field="title" :label="t('fields.case.title')" />
          <SortableHeader v-model:sort="sort" field="case_type" :label="t('fields.case.case_type')" />
          <SortableHeader v-model:sort="sort" field="status" :label="t('fields.case.status')" />
          <SortableHeader v-model:sort="sort" field="opened_at" :label="t('fields.case.opened_at')" />
        </tr>
      </thead>

      <tbody>
        <tr
          v-for="c in cases"
          :key="c.subjectRef?.id"
          :class="{ 'text-disabled': c.recordMetadata?.deletedAt }"
          role="link"
          style="cursor: pointer"
          tabindex="0"
          @click="openCase(c)"
          @keydown.enter="openCase(c)"
          @keydown.space.prevent="openCase(c)"
        >
          <td class="text-no-wrap">{{ c.subjectRef?.businessRef || '—' }}</td>

          <td>
            {{ c.title }}
            <v-icon v-if="c.recordMetadata?.isLocked" icon="mdi-lock" size="x-small" />
          </td>

          <td>{{ c.caseType?.label ?? c.caseType?.code }}</td>
          <td><CaseStatusChip :status="c.status" /></td>
          <td class="text-caption">{{ formatDateTime(c.openedAt) }}</td>
        </tr>

        <tr v-if="!loading && cases.length === 0">
          <td class="text-medium-emphasis text-center py-6" colspan="5">{{ t('messages.common.noData') }}</td>
        </tr>
      </tbody>
    </v-table>

    <PagedListFooter
      :capped="totalCapped"
      :count="cases.length"
      :loading="loading"
      :more="!!nextPageToken"
      :total="totalSize"
      @more="load(false)"
    />
  </v-card>
</template>
