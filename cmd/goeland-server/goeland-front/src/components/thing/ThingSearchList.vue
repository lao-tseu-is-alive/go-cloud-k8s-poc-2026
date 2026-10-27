<script setup lang="ts">
  import type { GoThing, SearchThingsParams } from '@/api/types'
  import { watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { searchThings } from '@/api/thingClient'
  import PagedListFooter from '@/components/core/PagedListFooter.vue'
  import RecordFilterFields from '@/components/core/RecordFilterFields.vue'
  import SearchFilterCard from '@/components/core/SearchFilterCard.vue'
  import SortableHeader from '@/components/core/SortableHeader.vue'
  import ThingTypeSelect from '@/components/thing/ThingTypeSelect.vue'
  import { usePagedSearch } from '@/composables/usePagedSearch'
  import { formatDateTime } from '@/utils/formatters'
  import { activeCount, emptyRecordFilters, RECORD_FILTER_KEYS } from '@/utils/routeFilters'

  // The thing list with its filters (GLD-074), on the thing page and in a tab
  // of the search results; the filters live in the URL.
  const emit = defineEmits<{ total: [size: number, capped: boolean] }>()
  const { t } = useI18n()
  const router = useRouter()

  type ThingFilters = Pick<SearchThingsParams, 'query' | 'thingTypeCode' | 'createdFrom' | 'createdTo' | 'createdBy' | 'includeDeleted'>
  const MORE = [...RECORD_FILTER_KEYS, 'includeDeleted'] as const
  function emptyFilters (): ThingFilters {
    return { query: '', thingTypeCode: undefined, ...emptyRecordFilters(), includeDeleted: false }
  }

  const { filters, sort, items: things, nextPageToken, totalSize, totalCapped, loading, load, search, reset } = usePagedSearch(
    async (params: ThingFilters) => {
      const res = await searchThings(params)
      return { ...res, items: res.things ?? [] }
    }, emptyFilters)
  // Each finished search reports its total (the tab count of the results page).
  watch(loading, busy => busy || emit('total', totalSize.value, totalCapped.value))

  function openThing (th: GoThing) {
    const id = th.subjectRef?.id
    if (id) router.push(`/things/${id}`)
  }

  /** The official identifier shown in the list: parcel number / EGRID or EGID. */
  function identifier (th: GoThing): string {
    if (th.parcel) return [th.parcel.parcelNumber, th.parcel.egrid].filter(Boolean).join(' · ')
    if (th.building?.egid) return `EGID ${th.building.egid}`
    return th.externalRef || '—'
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
        :title="t('pages.things.list.queryHint')"
        @keydown.enter="search"
      />
    </v-col>

    <v-col cols="12" md="4">
      <ThingTypeSelect v-model="filters.thingTypeCode" clearable density="compact" hide-details />
    </v-col>

    <template #more>
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
          <SortableHeader v-model:sort="sort" field="name" :label="t('fields.thing.name')" />
          <SortableHeader v-model:sort="sort" field="thing_type" :label="t('fields.thing.thing_type')" />
          <th scope="col">{{ t('fields.thing.identifiers') }}</th>
          <th scope="col">{{ t('fields.thing.area') }}</th>
          <SortableHeader v-model:sort="sort" field="created_at" :label="t('fields.thing.created_at')" />
        </tr>
      </thead>

      <tbody>
        <tr
          v-for="th in things"
          :key="th.subjectRef?.id"
          :class="{ 'text-disabled': th.recordMetadata?.deletedAt }"
          role="link"
          style="cursor: pointer"
          tabindex="0"
          @click="openThing(th)"
          @keydown.enter="openThing(th)"
          @keydown.space.prevent="openThing(th)"
        >
          <td>{{ th.name }}</td>
          <td>{{ th.thingType?.label ?? th.thingType?.code }}</td>
          <td>{{ identifier(th) }}</td>
          <td>{{ th.areaM2 ? `${Math.round(th.areaM2)} m²` : '—' }}</td>
          <td class="text-caption">{{ formatDateTime(th.createdAt) }}</td>
        </tr>

        <tr v-if="!loading && things.length === 0">
          <td class="text-medium-emphasis text-center py-6" colspan="5">{{ t('messages.common.noData') }}</td>
        </tr>
      </tbody>
    </v-table>

    <PagedListFooter
      :capped="totalCapped"
      :count="things.length"
      :loading="loading"
      :more="!!nextPageToken"
      :total="totalSize"
      @more="load(false)"
    />
  </v-card>
</template>
