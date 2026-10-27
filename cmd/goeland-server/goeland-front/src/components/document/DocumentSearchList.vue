<script setup lang="ts">
  import type { GoDocument, SearchDocumentsParams } from '@/api/types'
  import { watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { searchDocuments } from '@/api/documentClient'
  import DateRangeFields from '@/components/core/DateRangeFields.vue'
  import PagedListFooter from '@/components/core/PagedListFooter.vue'
  import RecordFilterFields from '@/components/core/RecordFilterFields.vue'
  import SearchFilterCard from '@/components/core/SearchFilterCard.vue'
  import SortableHeader from '@/components/core/SortableHeader.vue'
  import DocumentStatusChip from '@/components/document/DocumentStatusChip.vue'
  import DocumentTypeSelect from '@/components/document/DocumentTypeSelect.vue'
  import { usePagedSearch } from '@/composables/usePagedSearch'
  import { formatDate, formatDateTime } from '@/utils/formatters'
  import { activeCount, emptyRecordFilters, RECORD_FILTER_KEYS } from '@/utils/routeFilters'

  // The document list with its filters (GLD-074), on the document page and in
  // a tab of the search results; the filters live in the URL.
  const emit = defineEmits<{ total: [size: number, capped: boolean] }>()
  const { t } = useI18n()
  const router = useRouter()

  type DocumentFilters = Pick<SearchDocumentsParams, 'query' | 'documentTypeCode' | 'officialFrom' | 'officialTo' | 'createdFrom' | 'createdTo' | 'createdBy' | 'onlyFinal' | 'onlyRecords' | 'includeDeleted'>
  const MORE = ['officialFrom', 'officialTo', ...RECORD_FILTER_KEYS, 'onlyFinal', 'onlyRecords', 'includeDeleted'] as const
  function emptyFilters (): DocumentFilters {
    return { query: '', documentTypeCode: undefined, officialFrom: undefined, officialTo: undefined, ...emptyRecordFilters(), onlyFinal: false, onlyRecords: false, includeDeleted: false }
  }

  const { filters, sort, items: documents, nextPageToken, totalSize, totalCapped, loading, load, search, reset } = usePagedSearch(
    async (params: DocumentFilters) => {
      const res = await searchDocuments(params)
      return { ...res, items: res.documents ?? [] }
    }, emptyFilters)
  // Each finished search reports its total (the tab count of the results page).
  watch(loading, busy => busy || emit('total', totalSize.value, totalCapped.value))

  function openDocument (doc: GoDocument) {
    const id = doc.subjectRef?.id
    if (id) router.push(`/documents/${id}`)
  }
</script>

<template>
  <SearchFilterCard :more-count="activeCount(filters, MORE)" @reset="reset" @search="search">
    <v-col cols="12" md="8">
      <v-text-field
        v-model="filters.query"
        clearable
        density="compact"
        hide-details
        :label="t('fields.search.query')"
        prepend-inner-icon="mdi-magnify"
        @keydown.enter="search"
      />
    </v-col>

    <v-col cols="12" md="4">
      <DocumentTypeSelect v-model="filters.documentTypeCode" clearable density="compact" hide-details />
    </v-col>

    <template #more>
      <DateRangeFields v-model:from="filters.officialFrom" v-model:to="filters.officialTo" :label="t('filters.official')" @search="search" />
      <RecordFilterFields v-model="filters" @search="search" />

      <v-col class="d-flex flex-wrap ga-4" cols="12">
        <v-checkbox v-model="filters.onlyFinal" density="compact" hide-details :label="t('fields.document.only_final')" />
        <v-checkbox v-model="filters.onlyRecords" density="compact" hide-details :label="t('fields.document.only_records')" />
        <v-checkbox v-model="filters.includeDeleted" density="compact" hide-details :label="t('fields.document.include_deleted')" />
      </v-col>
    </template>
  </SearchFilterCard>

  <v-card>
    <v-table hover>
      <thead>
        <tr>
          <SortableHeader v-model:sort="sort" field="title" :label="t('fields.document.title')" />
          <SortableHeader v-model:sort="sort" field="document_type" :label="t('fields.document.document_type')" />
          <SortableHeader v-model:sort="sort" field="status" :label="t('fields.document.status')" />
          <SortableHeader v-model:sort="sort" field="official_date" :label="t('fields.document.official_date')" />
          <SortableHeader v-model:sort="sort" field="is_final" :label="t('fields.document.is_final')" />
          <SortableHeader v-model:sort="sort" field="created_at" :label="t('fields.common.created_at')" />
        </tr>
      </thead>

      <tbody>
        <tr
          v-for="doc in documents"
          :key="doc.subjectRef?.id"
          :class="{ 'text-disabled': doc.recordMetadata?.deletedAt }"
          style="cursor: pointer"
          tabindex="0"
          @click="openDocument(doc)"
          @keydown.enter="openDocument(doc)"
          @keydown.space.prevent="openDocument(doc)"
        >
          <td>
            <router-link class="list-link" :to="`/documents/${doc.subjectRef?.id}`" @click.stop>{{ doc.title }}</router-link>
            <v-icon v-if="doc.recordMetadata?.isLocked" icon="mdi-lock" size="x-small" />
          </td>

          <td>{{ doc.documentType?.label ?? doc.documentType?.code ?? '—' }}</td>
          <td><DocumentStatusChip :status="doc.status" /></td>
          <td>{{ formatDate(doc.officialDate) }}</td>

          <td>
            <v-icon
              :color="doc.currentVersion?.isFinal ? 'success' : undefined"
              :icon="doc.currentVersion?.isFinal ? 'mdi-check-circle' : 'mdi-circle-outline'"
              size="small"
            />
          </td>

          <td class="text-caption">{{ formatDateTime(doc.createdAt) }}</td>
        </tr>

        <tr v-if="!loading && documents.length === 0">
          <td class="text-medium-emphasis text-center py-6" colspan="6">{{ t('messages.common.noData') }}</td>
        </tr>
      </tbody>
    </v-table>

    <PagedListFooter
      :capped="totalCapped"
      :count="documents.length"
      :loading="loading"
      :more="!!nextPageToken"
      :total="totalSize"
      @more="load(false)"
    />
  </v-card>
</template>
