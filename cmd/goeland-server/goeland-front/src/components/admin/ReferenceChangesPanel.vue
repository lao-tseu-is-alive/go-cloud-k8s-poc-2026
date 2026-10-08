<script setup lang="ts">
  import type { ReferenceCatalogue, ReferenceChange } from '@/api/types'
  import type { ListSort } from '@/utils/listSort'
  import { onMounted, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { listReferenceChanges } from '@/api/referenceClient'
  import SortableHeader from '@/components/core/SortableHeader.vue'
  import UserLabel from '@/components/core/UserLabel.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { formatDateTime } from '@/utils/formatters'
  import { toOrderBy } from '@/utils/listSort'

  // Read-only view of the append-only reference change log, newest first
  // unless a header sorts it (on the server, GLD-056).
  const props = defineProps<{ catalogue?: ReferenceCatalogue }>()
  const { t } = useI18n()
  const { report } = useApiErrors()

  const changes = ref<ReferenceChange[]>([])
  const nextPageToken = ref('')
  const loading = ref(false)
  const sort = ref<ListSort>()

  async function load (reset = true) {
    loading.value = true
    try {
      const res = await listReferenceChanges({ catalogue: props.catalogue, orderBy: toOrderBy(sort.value), pageSize: 50, pageToken: reset ? undefined : nextPageToken.value })
      changes.value = reset ? (res.changes ?? []) : [...changes.value, ...(res.changes ?? [])]
      nextPageToken.value = res.nextPageToken ?? ''
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }
  onMounted(() => load())
  watch(sort, () => load())
  defineExpose({ reload: () => load() })
</script>

<template>
  <div>
    <p v-if="!loading && changes.length === 0" class="text-medium-emphasis">{{ t('messages.common.noData') }}</p>

    <v-table v-else density="compact">
      <thead>
        <tr>
          <SortableHeader v-model:sort="sort" field="occurred_at" :label="t('fields.reference.occurredAt')" />
          <SortableHeader v-model:sort="sort" field="catalogue" :label="t('fields.reference.catalogue')" />
          <SortableHeader v-model:sort="sort" field="code" :label="t('fields.reference.code')" />
          <SortableHeader v-model:sort="sort" field="event" :label="t('fields.reference.event')" />
          <th scope="col">{{ t('fields.audit.actor_user_id') }}</th>
          <th scope="col">{{ t('fields.reference.reason') }}</th>
          <th scope="col">{{ t('fields.audit.before_state') }} / {{ t('fields.audit.after_state') }}</th>
        </tr>
      </thead>

      <tbody>
        <tr v-for="change in changes" :key="change.id">
          <td class="text-caption">{{ formatDateTime(change.occurredAt) }}</td>
          <td>{{ t(`sections.reference.${change.catalogue}`) }}</td>
          <td><code>{{ change.code }}</code></td>
          <td>{{ t(`messages.reference.event.${change.eventType}`) }}</td>
          <td><UserLabel :id="change.actorUserId" /></td>
          <td>{{ change.reason || '—' }}</td>
          <td><pre class="state">{{ JSON.stringify({ before: change.beforeState, after: change.afterState }, null, 1) }}</pre></td>
        </tr>
      </tbody>
    </v-table>

    <div class="d-flex justify-center mt-2">
      <v-btn v-if="nextPageToken" :loading="loading" variant="text" @click="load(false)">{{ t('actions.common.loadMore') }}</v-btn>
    </div>
  </div>
</template>

<style scoped>
  .state {
    font-size: 0.7rem;
    max-width: 22rem;
    overflow-wrap: anywhere;
    white-space: pre-wrap;
  }
</style>
