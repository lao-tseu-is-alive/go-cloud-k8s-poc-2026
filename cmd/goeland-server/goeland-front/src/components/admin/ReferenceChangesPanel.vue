<script setup lang="ts">
  import type { ReferenceCatalogue, ReferenceChange } from '@/api/types'
  import { onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { listReferenceChanges } from '@/api/referenceClient'
  import UserLabel from '@/components/core/UserLabel.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { formatDateTime } from '@/utils/formatters'

  // Read-only view of the append-only reference change log, newest first.
  const props = defineProps<{ catalogue?: ReferenceCatalogue }>()
  const { t } = useI18n()
  const { report } = useApiErrors()

  const changes = ref<ReferenceChange[]>([])
  const nextPageToken = ref('')
  const loading = ref(false)

  async function load (reset = true) {
    loading.value = true
    try {
      const res = await listReferenceChanges({ catalogue: props.catalogue, pageSize: 50, pageToken: reset ? undefined : nextPageToken.value })
      changes.value = reset ? (res.changes ?? []) : [...changes.value, ...(res.changes ?? [])]
      nextPageToken.value = res.nextPageToken ?? ''
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }
  onMounted(() => load())
  defineExpose({ reload: () => load() })
</script>

<template>
  <div>
    <p v-if="!loading && changes.length === 0" class="text-medium-emphasis">{{ t('messages.common.noData') }}</p>

    <v-table v-else density="compact">
      <thead>
        <tr>
          <th scope="col">{{ t('fields.reference.occurredAt') }}</th>
          <th scope="col">{{ t('fields.reference.catalogue') }}</th>
          <th scope="col">{{ t('fields.reference.code') }}</th>
          <th scope="col">{{ t('fields.reference.event') }}</th>
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
