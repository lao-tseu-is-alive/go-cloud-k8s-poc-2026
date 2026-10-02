<script setup lang="ts">
  import type { GoActor, SearchActorsParams } from '@/api/types'
  import type { ListSort } from '@/utils/listSort'
  import { onMounted, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { searchActors } from '@/api/actorClient'
  import ActorSearchFilters from '@/components/actor/ActorSearchFilters.vue'
  import SortableHeader from '@/components/core/SortableHeader.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { formatDateTime, formatTotal } from '@/utils/formatters'
  import { toOrderBy } from '@/utils/listSort'

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const router = useRouter()
  const { report } = useApiErrors()

  const PAGE_SIZE = 25
  function emptyFilters (): SearchActorsParams {
    return { query: '', actorKind: undefined, organizationCategoryCode: undefined, onlyActive: false, includeDeleted: false }
  }
  const filters = ref<SearchActorsParams>(emptyFilters())
  const sort = ref<ListSort>()
  const actors = ref<GoActor[]>([])
  const nextPageToken = ref('')
  const totalSize = ref(0)
  const totalCapped = ref(false)
  const loading = ref(false)

  async function load (reset: boolean) {
    loading.value = true
    try {
      const res = await searchActors({
        ...filters.value,
        orderBy: toOrderBy(sort.value),
        pageSize: PAGE_SIZE,
        pageToken: reset ? undefined : nextPageToken.value || undefined,
      })
      const page = res.actors ?? []
      actors.value = reset ? page : [...actors.value, ...page]
      nextPageToken.value = res.nextPageToken ?? ''
      totalSize.value = res.totalSize ?? actors.value.length
      totalCapped.value = !!res.totalSizeCapped
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  function onSearch () {
    nextPageToken.value = ''
    void load(true)
  }
  // A click on a column header sorts on the server, from the first page.
  watch(sort, onSearch)

  function onReset () {
    filters.value = emptyFilters()
    onSearch()
  }

  function openActor (actor: GoActor) {
    const id = actor.subjectRef?.id
    if (id) router.push(`/actors/${id}`)
  }

  onMounted(() => load(true))
</script>

<template>
  <v-container fluid>
    <div class="d-flex align-center justify-space-between mb-4 flex-wrap ga-2">
      <h1 class="text-h5">{{ t('pages.actors.list.title') }}</h1>

      <v-btn color="primary" prepend-icon="mdi-plus" to="/actors/new" variant="flat">
        {{ t('nav.createActor') }}
      </v-btn>
    </div>

    <ActorSearchFilters v-model="filters" @reset="onReset" @search="onSearch" />

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
            role="link"
            style="cursor: pointer"
            tabindex="0"
            @click="openActor(actor)"
            @keydown.enter="openActor(actor)"
            @keydown.space.prevent="openActor(actor)"
          >
            <td>
              {{ actor.displayName }}
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

      <v-card-actions>
        <span class="text-caption text-medium-emphasis">{{ actors.length }} / {{ formatTotal(totalSize, totalCapped) }}</span>
        <v-spacer />

        <v-btn v-if="nextPageToken" :loading="loading" variant="text" @click="load(false)">
          {{ t('actions.common.refresh') }} +
        </v-btn>
      </v-card-actions>

      <v-progress-linear v-if="loading" color="primary" indeterminate />
    </v-card>
  </v-container>
</template>
