<script setup lang="ts">
  import type { ActorKind, GoActor, SearchActorsParams } from '@/api/types'
  import { computed, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { searchActors } from '@/api/actorClient'
  import { ACTOR_KINDS } from '@/components/actor/actorForm'
  import OrganizationCategorySelect from '@/components/actor/OrganizationCategorySelect.vue'
  import PagedListFooter from '@/components/core/PagedListFooter.vue'
  import RecordFilterFields from '@/components/core/RecordFilterFields.vue'
  import SearchFilterCard from '@/components/core/SearchFilterCard.vue'
  import SortableHeader from '@/components/core/SortableHeader.vue'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { usePagedSearch } from '@/composables/usePagedSearch'
  import { formatDateTime } from '@/utils/formatters'
  import { activeCount, emptyRecordFilters, RECORD_FILTER_KEYS } from '@/utils/routeFilters'

  // The actor list with its filters (GLD-074), on the actor page and in a tab
  // of the search results; the filters live in the URL.
  const emit = defineEmits<{ total: [size: number, capped: boolean] }>()
  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const router = useRouter()

  type ActorFilters = Pick<SearchActorsParams, 'query' | 'actorKind' | 'organizationCategoryCode' | 'createdFrom' | 'createdTo' | 'createdBy' | 'onlyActive' | 'includeDeleted'>
  const MORE = [...RECORD_FILTER_KEYS, 'onlyActive', 'includeDeleted'] as const
  function emptyFilters (): ActorFilters {
    return { query: '', actorKind: undefined, organizationCategoryCode: undefined, ...emptyRecordFilters(), onlyActive: false, includeDeleted: false }
  }

  const { filters, sort, items: actors, nextPageToken, totalSize, totalCapped, loading, load, search, reset } = usePagedSearch(
    async (params: ActorFilters) => {
      const res = await searchActors(params)
      return { ...res, items: res.actors ?? [] }
    }, emptyFilters)
  const kindItems = computed(() => ACTOR_KINDS.map((k: ActorKind) => ({ value: k, title: enumLabel('ActorKind', k) })))
  // Each finished search reports its total (the tab count of the results page).
  watch(loading, busy => busy || emit('total', totalSize.value, totalCapped.value))

  function openActor (actor: GoActor) {
    const id = actor.subjectRef?.id
    if (id) router.push(`/actors/${id}`)
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
        @keydown.enter="search"
      />
    </v-col>

    <v-col cols="12" md="3" sm="6">
      <v-select
        v-model="filters.actorKind"
        clearable
        density="compact"
        hide-details
        :items="kindItems"
        :label="t('fields.actor.actor_kind')"
      />
    </v-col>

    <v-col cols="12" md="3" sm="6">
      <OrganizationCategorySelect v-model="filters.organizationCategoryCode" clearable density="compact" hide-details />
    </v-col>

    <template #more>
      <RecordFilterFields v-model="filters" @search="search" />

      <v-col class="d-flex flex-wrap ga-4" cols="12" md="4">
        <v-checkbox v-model="filters.onlyActive" density="compact" hide-details :label="t('fields.actor.only_active')" />
        <v-checkbox v-model="filters.includeDeleted" density="compact" hide-details :label="t('fields.document.include_deleted')" />
      </v-col>
    </template>
  </SearchFilterCard>

  <v-card>
    <v-table hover>
      <thead>
        <tr>
          <SortableHeader v-model:sort="sort" field="display_name" :label="t('fields.actor.display_name')" />
          <SortableHeader v-model:sort="sort" field="actor_kind" :label="t('fields.actor.actor_kind')" />
          <SortableHeader v-model:sort="sort" field="category" :label="t('fields.actor.category')" />
          <SortableHeader v-model:sort="sort" field="is_active" :label="t('fields.actor.is_active')" />
          <SortableHeader v-model:sort="sort" field="created_at" :label="t('fields.common.created_at')" />
        </tr>
      </thead>

      <tbody>
        <tr
          v-for="actor in actors"
          :key="actor.subjectRef?.id"
          :class="{ 'text-disabled': actor.recordMetadata?.deletedAt }"
          style="cursor: pointer"
          tabindex="0"
          @click="openActor(actor)"
          @keydown.enter="openActor(actor)"
          @keydown.space.prevent="openActor(actor)"
        >
          <td>
            <router-link class="list-link" :to="`/actors/${actor.subjectRef?.id}`" @click.stop>{{ actor.displayName }}</router-link>
            <v-icon v-if="actor.recordMetadata?.isLocked" icon="mdi-lock" size="x-small" />
          </td>

          <td>
            <v-chip
              :color="actor.actorKind === 'ACTOR_KIND_ORGANIZATION' ? 'indigo' : 'teal'"
              label
              size="small"
            >
              {{ enumLabel('ActorKind', actor.actorKind) }}
            </v-chip>
          </td>

          <td>{{ actor.organization?.categoryCode ?? '—' }}</td>

          <td>
            <v-icon
              :color="actor.isActive ? 'success' : 'grey'"
              :icon="actor.isActive ? 'mdi-check-circle' : 'mdi-circle-outline'"
              size="small"
            />
          </td>

          <td class="text-caption">{{ formatDateTime(actor.createdAt) }}</td>
        </tr>

        <tr v-if="!loading && actors.length === 0">
          <td class="text-medium-emphasis text-center py-6" colspan="5">{{ t('messages.common.noData') }}</td>
        </tr>
      </tbody>
    </v-table>

    <PagedListFooter
      :capped="totalCapped"
      :count="actors.length"
      :loading="loading"
      :more="!!nextPageToken"
      :total="totalSize"
      @more="load(false)"
    />
  </v-card>
</template>
