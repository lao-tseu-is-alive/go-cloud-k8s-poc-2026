<script setup lang="ts">
  import type { BordereauStatus, DispositionBordereau } from '@/api/types'
  import { onMounted, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { listDispositionBordereaux } from '@/api/retentionClient'
  import UserLabel from '@/components/core/UserLabel.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { formatDateTime } from '@/utils/formatters'
  import { bordereauStatusColor, dispositionColor } from '@/utils/retention'

  // The disposition bordereaux, newest first, each opening its detail page.
  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const { report } = useApiErrors()

  const STATUSES: BordereauStatus[] = ['BORDEREAU_STATUS_OPEN', 'BORDEREAU_STATUS_EXECUTED', 'BORDEREAU_STATUS_CANCELLED']
  const status = ref<BordereauStatus | null>('BORDEREAU_STATUS_OPEN')
  const rows = ref<DispositionBordereau[]>([])
  const nextToken = ref('')
  const loading = ref(false)

  async function load (append = false) {
    loading.value = true
    try {
      const res = await listDispositionBordereaux({ status: status.value ?? undefined, pageSize: 50, pageToken: append ? nextToken.value : undefined })
      rows.value = append ? [...rows.value, ...(res.bordereaux ?? [])] : (res.bordereaux ?? [])
      nextToken.value = res.nextPageToken ?? ''
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }
  watch(status, () => load())
  onMounted(load)
</script>

<template>
  <div>
    <v-chip-group v-model="status" class="mb-2" filter>
      <v-chip
        v-for="s in STATUSES"
        :key="s"
        size="small"
        :value="s"
        variant="outlined"
      >{{ enumLabel('BordereauStatus', s) }}</v-chip>
    </v-chip-group>

    <v-progress-linear v-if="loading" class="mb-2" color="primary" indeterminate />
    <p v-if="!loading && rows.length === 0" class="text-medium-emphasis">{{ t('retention.bordereau.empty') }}</p>

    <v-table v-else density="compact">
      <thead>
        <tr>
          <th scope="col">{{ t('retention.bordereau.preparedAt') }}</th>
          <th scope="col">{{ t('retention.page.disposition') }}</th>
          <th scope="col">{{ t('retention.bordereau.reason') }}</th>
          <th scope="col">{{ t('retention.bordereau.cases') }}</th>
          <th scope="col">{{ t('retention.bordereau.status') }}</th>
        </tr>
      </thead>

      <tbody>
        <tr v-for="b in rows" :key="b.id">
          <td class="text-caption">
            <router-link :to="`/retention/bordereaux/${b.id}`">{{ formatDateTime(b.preparedAt) }}</router-link>
            <div><UserLabel :id="b.preparedBy" /></div>
          </td>

          <td><v-chip :color="dispositionColor(b.finalDisposition)" label size="x-small">{{ enumLabel('FinalDisposition', b.finalDisposition) }}</v-chip></td>
          <td>{{ b.reason }}</td>
          <td>{{ t('retention.bordereau.counts', { total: b.itemCount ?? 0, pending: b.pendingCount ?? 0, approved: b.approvedCount ?? 0 }) }}</td>
          <td><v-chip :color="bordereauStatusColor(b.status)" label size="x-small">{{ enumLabel('BordereauStatus', b.status) }}</v-chip></td>
        </tr>
      </tbody>
    </v-table>

    <div v-if="nextToken" class="d-flex justify-center mt-2">
      <v-btn :loading="loading" variant="text" @click="load(true)">{{ t('actions.common.loadMore') }}</v-btn>
    </div>
  </div>
</template>
