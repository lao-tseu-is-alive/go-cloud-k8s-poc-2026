<script setup lang="ts">
  import type { CaseStatus, GoCase, SearchCasesParams } from '@/api/types'
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRouter } from 'vue-router'
  import { searchCases } from '@/api/caseClient'
  import { CASE_STATUSES } from '@/components/case/caseForm'
  import CaseStatusChip from '@/components/case/CaseStatusChip.vue'
  import CaseTypeSelect from '@/components/case/CaseTypeSelect.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { formatDateTime } from '@/utils/formatters'

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const router = useRouter()
  const { report } = useApiErrors()

  const PAGE_SIZE = 25
  function emptyFilters (): SearchCasesParams {
    return { query: '', caseTypeCode: undefined, status: undefined, includeDeleted: false }
  }
  const filters = ref<SearchCasesParams>(emptyFilters())
  const cases = ref<GoCase[]>([])
  const nextPageToken = ref('')
  const totalSize = ref(0)
  const loading = ref(false)
  const statusItems = computed(() => CASE_STATUSES.map((s: CaseStatus) => ({ value: s, title: enumLabel('CaseStatus', s) })))

  async function load (reset: boolean) {
    loading.value = true
    try {
      const res = await searchCases({
        ...filters.value,
        pageSize: PAGE_SIZE,
        pageToken: reset ? undefined : nextPageToken.value || undefined,
      })
      const page = res.cases ?? []
      cases.value = reset ? page : [...cases.value, ...page]
      nextPageToken.value = res.nextPageToken ?? ''
      totalSize.value = res.totalSize ?? cases.value.length
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

  function onReset () {
    filters.value = emptyFilters()
    onSearch()
  }

  function openCase (c: GoCase) {
    const id = c.subjectRef?.id
    if (id) router.push(`/cases/${id}`)
  }

  onMounted(() => load(true))
</script>

<template>
  <v-container fluid>
    <div class="d-flex align-center justify-space-between mb-4 flex-wrap ga-2">
      <h1 class="text-h5">{{ t('pages.cases.list.title') }}</h1>

      <v-btn color="primary" prepend-icon="mdi-plus" to="/cases/new" variant="flat">
        {{ t('nav.createCase') }}
      </v-btn>
    </div>

    <v-card class="mb-4">
      <v-card-text>
        <v-row dense>
          <v-col cols="12" md="5">
            <v-text-field
              v-model="filters.query"
              clearable
              density="compact"
              :hint="t('pages.cases.list.queryHint')"
              :label="t('search.query')"
              prepend-inner-icon="mdi-magnify"
              @keydown.enter="onSearch"
            />
          </v-col>

          <v-col cols="12" md="3">
            <CaseTypeSelect v-model="filters.caseTypeCode" clearable density="compact" />
          </v-col>

          <v-col cols="12" md="2">
            <v-select
              v-model="filters.status"
              clearable
              density="compact"
              :items="statusItems"
              :label="t('fields.case.status')"
            />
          </v-col>

          <v-col class="d-flex align-center ga-2" cols="12" md="2">
            <v-btn color="primary" variant="tonal" @click="onSearch">{{ t('actions.common.search') }}</v-btn>
            <v-btn variant="text" @click="onReset">{{ t('actions.common.reset') }}</v-btn>
          </v-col>
        </v-row>
      </v-card-text>
    </v-card>

    <v-card>
      <v-table hover>
        <thead>
          <tr>
            <th scope="col">{{ t('fields.subject.business_ref') }}</th>
            <th scope="col">{{ t('fields.case.title') }}</th>
            <th scope="col">{{ t('fields.case.case_type') }}</th>
            <th scope="col">{{ t('fields.case.status') }}</th>
            <th scope="col">{{ t('fields.case.opened_at') }}</th>
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

      <v-card-actions>
        <span class="text-caption text-medium-emphasis">{{ cases.length }} / {{ totalSize }}</span>
        <v-spacer />

        <v-btn v-if="nextPageToken" :loading="loading" variant="text" @click="load(false)">
          {{ t('actions.common.refresh') }} +
        </v-btn>
      </v-card-actions>

      <v-progress-linear v-if="loading" color="primary" indeterminate />
    </v-card>
  </v-container>
</template>
