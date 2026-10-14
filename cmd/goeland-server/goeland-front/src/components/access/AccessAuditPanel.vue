<script setup lang="ts">
  import type { AccessAuditAction, AccessAuditEvent, AccessAuditOutcome } from '@/api/types'
  import type { ListSort } from '@/utils/listSort'
  import { ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { listAccessAuditEvents } from '@/api/accessClient'
  import { LEVEL_COLORS } from '@/components/access/grantees'
  import SortableHeader from '@/components/core/SortableHeader.vue'
  import SubjectLink from '@/components/core/SubjectLink.vue'
  import UserLabel from '@/components/core/UserLabel.vue'
  import UserPicker from '@/components/core/UserPicker.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { formatDateTime, formatTotal } from '@/utils/formatters'
  import { toOrderBy } from '@/utils/listSort'
  import { kindIcon } from '@/utils/subjects'

  // The sensitive read audit (GLD-033): reads and downloads of confidential
  // subjects, allowed or refused, newest first. With a subjectId it shows that
  // subject's log (holders of FULL_CONTROL); without, the whole log with
  // filters (administrators). A subject the caller cannot read comes without
  // its label and without a link (opening it would only log a refusal).
  const props = defineProps<{ subjectId?: string }>()
  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const { report } = useApiErrors()

  const ACTIONS: AccessAuditAction[] = ['READ_SENSITIVE', 'DOWNLOAD', 'EXPORT']
  const OUTCOMES: AccessAuditOutcome[] = ['GRANTED', 'DENIED']

  const events = ref<AccessAuditEvent[]>([])
  const nextPageToken = ref('')
  const total = ref<number>()
  const capped = ref(false)
  const loading = ref(false)
  const sort = ref<ListSort>()
  const userId = ref<string>()
  const action = ref<AccessAuditAction>()
  const outcome = ref<AccessAuditOutcome>()

  async function load (reset = true) {
    loading.value = true
    try {
      const res = await listAccessAuditEvents({
        subjectId: props.subjectId,
        userId: userId.value,
        action: action.value,
        outcome: outcome.value,
        orderBy: toOrderBy(sort.value),
        pageSize: 50,
        pageToken: reset ? undefined : nextPageToken.value,
      })
      events.value = reset ? (res.events ?? []) : [...events.value, ...(res.events ?? [])]
      nextPageToken.value = res.nextPageToken ?? ''
      total.value = res.totalSize ?? 0
      capped.value = res.totalSizeCapped ?? false
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  /** The subject as a link target, only when its label was disclosed. */
  function linkable (e: AccessAuditEvent) {
    if (!e.subjectLabel || !e.subjectKind) return undefined
    const label = e.businessRef ? `${e.businessRef} · ${e.subjectLabel}` : e.subjectLabel
    return { id: e.subjectId, kind: e.subjectKind, displayLabel: label }
  }

  watch([() => props.subjectId, sort, userId, action, outcome], () => load(), { immediate: true })
</script>

<template>
  <div>
    <div v-if="!subjectId" class="d-flex flex-wrap ga-3 mb-2">
      <div class="filter"><UserPicker v-model="userId" :label="t('access.audit.reader')" /></div>

      <v-select
        v-model="action"
        class="filter"
        clearable
        :item-title="(a: AccessAuditAction) => enumLabel('AccessAuditAction', a)"
        :item-value="(a: AccessAuditAction) => a"
        :items="ACTIONS"
        :label="t('access.audit.action')"
      />

      <v-select
        v-model="outcome"
        class="filter"
        clearable
        :item-title="(o: AccessAuditOutcome) => enumLabel('AccessAuditOutcome', o)"
        :item-value="(o: AccessAuditOutcome) => o"
        :items="OUTCOMES"
        :label="t('access.audit.outcome')"
      />
    </div>

    <p class="text-caption text-medium-emphasis mb-2">{{ t('access.audit.hint') }}</p>
    <v-progress-linear v-if="loading" class="mb-2" color="primary" indeterminate />
    <p v-if="!loading && events.length === 0" class="text-medium-emphasis">{{ t('access.audit.empty') }}</p>

    <template v-else-if="events.length > 0">
      <p class="text-caption mb-1">{{ t('access.audit.total', { total: formatTotal(total, capped) }) }}</p>

      <v-table density="compact">
        <thead>
          <tr>
            <SortableHeader v-model:sort="sort" field="occurred_at" :label="t('access.audit.occurredAt')" />
            <th scope="col">{{ t('access.audit.reader') }}</th>
            <th v-if="!subjectId" scope="col">{{ t('access.audit.subject') }}</th>
            <th scope="col">{{ t('access.audit.action') }}</th>
            <th scope="col">{{ t('access.audit.outcome') }}</th>
            <th scope="col">{{ t('access.audit.level') }}</th>
          </tr>
        </thead>

        <tbody>
          <tr v-for="e in events" :key="e.id">
            <td class="text-caption">{{ formatDateTime(e.occurredAt) }}</td>
            <td><UserLabel :id="e.userId" /></td>

            <td v-if="!subjectId">
              <SubjectLink v-if="linkable(e)" :subject="linkable(e)" />

              <span v-else class="text-medium-emphasis" :title="t('access.audit.hiddenLabel')">
                <v-icon class="mr-1" :icon="kindIcon(e.subjectKind)" size="small" />{{ e.businessRef || e.subjectId }}
              </span>
            </td>

            <td>
              {{ enumLabel('AccessAuditAction', e.action) }}
              <span v-if="e.documentVersionId" class="text-caption text-medium-emphasis d-block">{{ t('access.audit.version', { id: e.documentVersionId.slice(0, 8) }) }}</span>
            </td>

            <td>
              <v-chip :color="e.outcome === 'DENIED' ? 'error' : 'success'" label size="x-small">{{ enumLabel('AccessAuditOutcome', e.outcome) }}</v-chip>
            </td>

            <td>
              <v-chip :color="LEVEL_COLORS[e.accessLevel ?? 'PERMISSION_NONE']" label size="x-small">{{ enumLabel('Permission', e.accessLevel) }}</v-chip>
              <span class="text-caption text-medium-emphasis d-block">{{ enumLabel('AccessSource', e.accessSource) }} · {{ t('access.audit.confidentiality', { level: e.confidentialityLevel ?? 0 }) }}</span>
            </td>
          </tr>
        </tbody>
      </v-table>
    </template>

    <div class="d-flex justify-center mt-2">
      <v-btn v-if="nextPageToken" :loading="loading" variant="text" @click="load(false)">{{ t('actions.common.loadMore') }}</v-btn>
    </div>
  </div>
</template>

<style scoped>
  .filter {
    flex: 1 1 14rem;
    max-width: 22rem;
  }
</style>
