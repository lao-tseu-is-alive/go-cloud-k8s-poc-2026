<script setup lang="ts">
  import type { RelationshipFamily, RelationshipGroup, SubjectRef, SubjectRelationship } from '@/api/types'
  import { computed, onBeforeUnmount, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { listRelationshipsPage } from '@/api/coreClient'
  import SubjectLink from '@/components/core/SubjectLink.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { formatDate } from '@/utils/formatters'
  import { GROUP_FILTER_THRESHOLD } from '@/utils/roleHeadings'

  // One relationship group of a detail page (GLD-063): a heading read from the
  // subject on screen and its count, unfolded on demand into its edges. A case
  // group filters by state; past GROUP_FILTER_THRESHOLD edges any group filters
  // by text, by period (cases, documents) and by type when it has several
  // (case types, document families, thing types). A document or thing group
  // with several types unfolds into one nested group per type until a filter
  // is set, then lists the matching edges. Rows load page by page when opened.
  const props = defineProps<{
    subjectId: string
    group: RelationshipGroup
    heading: string
    /** The family this nested group shows (document type); none for a top group. */
    family?: RelationshipFamily
  }>()
  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const { report } = useApiErrors()

  const PAGE_SIZE = 25

  const open = ref(false)
  const rows = ref<SubjectRelationship[]>([])
  const nextPageToken = ref('')
  const total = ref(0)
  const loading = ref(false)
  const status = ref<'' | 'open' | 'closed'>('')
  const typeCode = ref('')
  const period = ref('')
  const query = ref('')
  const appliedQuery = ref('')
  let queryTimer: ReturnType<typeof setTimeout> | undefined

  const kind = computed(() => props.group.otherKind)
  const count = computed(() => props.family?.count ?? props.group.count ?? 0)
  const openCount = computed(() => props.family?.openCount ?? props.group.openCount ?? 0)
  const isCases = computed(() => kind.value === 'SUBJECT_KIND_CASE')
  const families = computed(() => props.group.families ?? [])
  const several = computed(() => !props.family && families.value.length > 1)
  const filtering = computed(() => !!(appliedQuery.value || typeCode.value || period.value))
  const showFilters = computed(() => count.value > GROUP_FILTER_THRESHOLD)
  const dated = computed(() => isCases.value || kind.value === 'SUBJECT_KIND_DOCUMENT')
  const nested = computed(() => several.value && !filtering.value
    && (kind.value === 'SUBJECT_KIND_DOCUMENT' || kind.value === 'SUBJECT_KIND_THING'))
  // the type column only when the group mixes types and the rows are not already one type
  const typeColumn = computed(() => several.value && !typeCode.value)

  const year = new Date().getFullYear()
  const periods = computed(() => [
    { value: '', title: t('relGroups.filters.anyPeriod') },
    { value: `${year}-${year}`, title: String(year) },
    { value: `${year - 1}-${year - 1}`, title: String(year - 1) },
    { value: `${year - 5}-${year - 2}`, title: `${year - 5} – ${year - 2}` },
    { value: `0-${year - 6}`, title: t('relGroups.filters.before', { year: year - 5 }) },
  ])
  const typeItems = computed(() => [
    { value: '', title: t('relGroups.filters.anyType') },
    ...families.value.map(f => ({ value: f.code, title: `${f.label || f.code} (${f.count ?? 0})` })),
  ])

  /** The other end of an edge, seen from the subject on screen. */
  function other (rel: SubjectRelationship): SubjectRef | undefined {
    return props.group.outgoing ? rel.target : rel.source
  }

  /** A business reference with its namespace (two namespaces may share a number). */
  function reference (ref?: SubjectRef): string {
    if (!ref?.businessRef) return '—'
    return ref.businessRefNamespace ? `${ref.businessRefNamespace} ${ref.businessRef}` : ref.businessRef
  }

  async function load (reset: boolean) {
    loading.value = true
    try {
      const [from, to] = period.value ? period.value.split('-').map(Number) : [0, 0]
      const res = await listRelationshipsPage(props.subjectId, {
        outgoing: props.group.outgoing ?? false,
        relationshipTypeCode: props.group.relationshipType?.code,
        includeDetails: true,
        otherCaseStatus: status.value,
        otherTypeCode: props.family?.code ?? typeCode.value,
        otherYearFrom: from,
        otherYearTo: to,
        otherQuery: appliedQuery.value,
        pageSize: PAGE_SIZE,
        pageToken: reset ? undefined : nextPageToken.value,
      })
      rows.value = reset ? (res.relationships ?? []) : [...rows.value, ...(res.relationships ?? [])]
      nextPageToken.value = res.nextPageToken ?? ''
      total.value = res.totalSize ?? 0
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  function toggle () {
    open.value = !open.value
    if (open.value && !nested.value && rows.value.length === 0) {
      void load(true)
    }
  }

  watch([status, typeCode, period, appliedQuery], () => {
    if (open.value && !nested.value) void load(true)
  })
  // the text filter waits for a pause in typing
  watch(query, value => {
    clearTimeout(queryTimer)
    queryTimer = setTimeout(() => {
      appliedQuery.value = value.trim()
    }, 300)
  })
  onBeforeUnmount(() => clearTimeout(queryTimer))
</script>

<template>
  <div class="role-group">
    <button
      :aria-expanded="open ? 'true' : 'false'"
      class="role-group__head"
      :class="{ 'role-group__head--open': open, 'role-group__head--nested': !!family }"
      type="button"
      @click="toggle"
    >
      <v-icon class="role-group__chevron" :class="{ 'role-group__chevron--open': open }" icon="mdi-chevron-right" size="small" />
      <span class="flex-grow-1">{{ heading }}</span>

      <v-chip
        v-if="isCases && openCount > 0"
        color="primary"
        label
        size="x-small"
        :title="t('relGroups.activeHint')"
      >{{ t('relGroups.active', { n: openCount }) }}</v-chip>

      <span class="role-group__count">{{ count }}</span>
    </button>

    <div v-if="open" class="role-group__body" :class="{ 'role-group__body--nested': !!family }">
      <div v-if="(isCases && !family) || showFilters" class="d-flex flex-wrap align-center ga-3 mb-2">
        <v-text-field
          v-if="showFilters"
          v-model="query"
          class="role-group__filter"
          clearable
          density="compact"
          hide-details
          :label="t('relGroups.filters.text')"
          prepend-inner-icon="mdi-magnify"
        />

        <v-btn-toggle
          v-if="isCases && !family"
          v-model="status"
          color="primary"
          density="compact"
          divided
          mandatory
          variant="outlined"
        >
          <v-btn size="small" value="">{{ t('relGroups.filters.all') }}</v-btn>
          <v-btn size="small" value="open">{{ t('relGroups.filters.active', { n: openCount }) }}</v-btn>
          <v-btn size="small" value="closed">{{ t('relGroups.filters.closed') }}</v-btn>
        </v-btn-toggle>

        <template v-if="showFilters">
          <v-select
            v-if="several"
            v-model="typeCode"
            class="role-group__filter"
            density="compact"
            hide-details
            :items="typeItems"
            :label="t(`relGroups.filters.type.${kind}`, t('relGroups.filters.type.other'))"
          />

          <v-select
            v-if="dated"
            v-model="period"
            class="role-group__filter"
            density="compact"
            hide-details
            :items="periods"
            :label="t('relGroups.filters.period')"
          />
        </template>
      </div>

      <template v-if="nested">
        <div class="role-group__families">
          <RoleGroup
            v-for="f in families"
            :key="f.code"
            :family="f"
            :group="group"
            :heading="f.label || f.code"
            :subject-id="subjectId"
          />
        </div>
      </template>

      <template v-else>
        <v-progress-linear v-if="loading && rows.length === 0" color="primary" indeterminate />
        <p v-else-if="rows.length === 0" class="text-medium-emphasis text-body-2 my-2">{{ t('relGroups.noMatch') }}</p>

        <v-table v-else density="compact">
          <thead>
            <tr>
              <th v-if="isCases" scope="col">{{ t('relGroups.columns.reference') }}</th>
              <th v-if="kind === 'SUBJECT_KIND_DOCUMENT'" scope="col">{{ t('relGroups.columns.date') }}</th>
              <th scope="col">{{ t(`relGroups.columns.subject.${kind}`, t('relGroups.columns.subject.other')) }}</th>
              <th v-if="typeColumn" scope="col">{{ t('relGroups.columns.type') }}</th>
              <th v-if="isCases" scope="col">{{ t('relGroups.columns.state') }}</th>
            </tr>
          </thead>

          <tbody>
            <tr v-for="rel in rows" :key="rel.id">
              <td v-if="isCases" class="role-group__mono">{{ reference(other(rel)) }}</td>
              <td v-if="kind === 'SUBJECT_KIND_DOCUMENT'" class="role-group__mono">{{ formatDate(rel.otherDetail?.date) }}</td>

              <td>
                <SubjectLink :subject="other(rel)" />
                <span v-if="rel.roleDetail" class="text-caption text-medium-emphasis"> — {{ rel.roleDetail }}</span>
                <span v-if="rel.validTo" class="text-caption text-medium-emphasis d-block">{{ t('relGroups.endedOn', { date: formatDate(rel.validTo) }) }}</span>
              </td>

              <td v-if="typeColumn">{{ rel.otherDetail?.typeLabel || '—' }}</td>

              <td v-if="isCases">
                <v-chip
                  v-if="rel.otherDetail?.caseStatus"
                  :color="rel.otherDetail.caseStatus === 'CLOSED' ? 'default' : 'primary'"
                  label
                  size="x-small"
                >{{ enumLabel('CaseStatus', `CASE_STATUS_${rel.otherDetail.caseStatus}`) }}</v-chip>
              </td>
            </tr>
          </tbody>
        </v-table>

        <div v-if="rows.length > 0" class="d-flex flex-wrap align-center ga-3 mt-2 text-body-2">
          <v-btn
            v-if="nextPageToken"
            :loading="loading"
            size="small"
            variant="text"
            @click="load(false)"
          >{{ t('actions.common.loadMore') }}</v-btn>

          <span class="text-medium-emphasis">{{ t('relGroups.shown', { shown: rows.length, total }) }}</span>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
  .role-group {
    border-top: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
  }

  .role-group__head {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 48px;
    padding: 8px 16px;
    background: none;
    border: none;
    cursor: pointer;
    text-align: left;
    color: inherit;
    font: inherit;
    font-weight: 500;
  }

  .role-group__head--nested {
    min-height: 44px;
    font-weight: 400;
  }

  .role-group__head--open,
  .role-group__head:hover {
    background: rgba(var(--v-theme-primary), 0.05);
  }

  .role-group__head:focus-visible {
    outline: 2px solid rgb(var(--v-theme-primary));
    outline-offset: -2px;
  }

  .role-group__chevron {
    transition: transform 0.15s;
  }

  .role-group__chevron--open {
    transform: rotate(90deg);
  }

  .role-group__count {
    min-width: 44px;
    text-align: right;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
  }

  .role-group__body {
    padding: 4px 16px 16px 44px;
  }

  .role-group__body--nested {
    padding: 0 12px 12px 36px;
  }

  .role-group__families {
    border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
    border-radius: 8px;
    overflow: hidden;
  }

  .role-group__filter {
    flex: 0 1 240px;
    min-width: 180px;
  }

  .role-group__mono {
    font-family: ui-monospace, monospace;
    font-size: 0.85rem;
    white-space: nowrap;
  }
</style>
