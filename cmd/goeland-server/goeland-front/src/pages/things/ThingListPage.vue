<script setup lang="ts">
  import type { GoThing, SearchThingsParams } from '@/api/types'
  import { onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { searchThings } from '@/api/thingClient'
  import ThingTypeSelect from '@/components/thing/ThingTypeSelect.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { formatDateTime } from '@/utils/formatters'

  const { t } = useI18n()
  const router = useRouter()
  const { report } = useApiErrors()

  const PAGE_SIZE = 25
  const filters = ref<SearchThingsParams>({ query: '', thingTypeCode: undefined })
  const things = ref<GoThing[]>([])
  const nextPageToken = ref('')
  const totalSize = ref(0)
  const loading = ref(false)

  async function load (reset: boolean) {
    loading.value = true
    try {
      const res = await searchThings({ ...filters.value, pageSize: PAGE_SIZE, pageToken: reset ? undefined : nextPageToken.value || undefined })
      const page = res.things ?? []
      things.value = reset ? page : [...things.value, ...page]
      nextPageToken.value = res.nextPageToken ?? ''
      totalSize.value = res.totalSize ?? things.value.length
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  function onReset () {
    filters.value = { query: '', thingTypeCode: undefined }
    void load(true)
  }

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

  onMounted(() => load(true))
</script>

<template>
  <v-container fluid>
    <div class="d-flex align-center justify-space-between mb-4 flex-wrap ga-2">
      <h1 class="text-h5">{{ t('pages.things.list.title') }}</h1>

      <v-btn color="primary" prepend-icon="mdi-plus" to="/things/new" variant="flat">
        {{ t('nav.createThing') }}
      </v-btn>
    </div>

    <v-card class="mb-4">
      <v-card-text>
        <v-row dense>
          <v-col cols="12" md="6">
            <v-text-field
              v-model="filters.query"
              clearable
              :hint="t('pages.things.list.queryHint')"
              :label="t('fields.search.query')"
              persistent-hint
              prepend-inner-icon="mdi-magnify"
              @keyup.enter="load(true)"
            />
          </v-col>

          <v-col cols="12" md="3">
            <ThingTypeSelect v-model="filters.thingTypeCode" clearable />
          </v-col>

          <v-col class="d-flex align-center ga-2" cols="12" md="3">
            <v-btn color="primary" variant="tonal" @click="load(true)">{{ t('actions.common.search') }}</v-btn>
            <v-btn variant="text" @click="onReset">{{ t('actions.common.reset') }}</v-btn>
          </v-col>
        </v-row>
      </v-card-text>
    </v-card>

    <v-card>
      <v-progress-linear v-if="loading" indeterminate />

      <v-table density="comfortable">
        <thead>
          <tr>
            <th scope="col">{{ t('fields.thing.name') }}</th>
            <th scope="col">{{ t('fields.thing.thing_type') }}</th>
            <th scope="col">{{ t('fields.thing.identifiers') }}</th>
            <th scope="col">{{ t('fields.thing.area') }}</th>
            <th scope="col">{{ t('fields.thing.created_at') }}</th>
          </tr>
        </thead>

        <tbody>
          <tr v-if="!loading && things.length === 0">
            <td class="text-medium-emphasis text-center" colspan="5">{{ t('messages.common.noData') }}</td>
          </tr>

          <tr
            v-for="th in things"
            :key="th.subjectRef?.id"
            class="clickable"
            tabindex="0"
            @click="openThing(th)"
            @keydown.enter="openThing(th)"
          >
            <td>{{ th.name }}</td>
            <td>{{ th.thingType?.label ?? th.thingType?.code }}</td>
            <td>{{ identifier(th) }}</td>
            <td>{{ th.areaM2 ? `${Math.round(th.areaM2)} m²` : '—' }}</td>
            <td class="text-caption">{{ formatDateTime(th.createdAt) }}</td>
          </tr>
        </tbody>
      </v-table>

      <div class="d-flex align-center justify-space-between pa-2">
        <span class="text-medium-emphasis">{{ things.length }} / {{ totalSize }}</span>
        <v-btn v-if="nextPageToken" :loading="loading" variant="text" @click="load(false)">{{ t('actions.common.loadMore') }}</v-btn>
      </div>
    </v-card>
  </v-container>
</template>

<style scoped>
  .clickable {
    cursor: pointer;
  }
</style>
