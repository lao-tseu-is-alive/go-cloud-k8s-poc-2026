<script setup lang="ts">
  import type { DispositionBordereau, DispositionItem } from '@/api/types'
  import { storeToRefs } from 'pinia'
  import { computed, onBeforeUnmount, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute } from 'vue-router'
  import { cancelDispositionBordereau, downloadTransferPackage, executeDispositionBordereau, getDispositionBordereau } from '@/api/retentionClient'
  import UserLabel from '@/components/core/UserLabel.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { useAuthStore } from '@/stores/auth'
  import { useUiStore } from '@/stores/ui'
  import { formatDate, formatDateTime } from '@/utils/formatters'
  import { bordereauStatusColor, decisionColor, dispositionColor } from '@/utils/retention'
  import { maxLength, required } from '@/utils/validation'

  // A disposition bordereau (GLD-032): its cases with their approvals (by the
  // owning unit or the case), the execution once all are decided, the transfer
  // packages to download and the proof of an executed bordereau.
  const route = useRoute()
  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const { report } = useApiErrors()
  const ui = useUiStore()
  const { isRecordsManager } = storeToRefs(useAuthStore())
  const POLL_MS = 3000

  const id = computed(() => String(route.params.id))
  const b = ref<DispositionBordereau | null>(null)
  const loading = ref(false)
  const busy = ref(false)
  const cancelling = ref(false)
  const cancelReason = ref('')
  const cancelForm = ref()
  const downloading = ref('')
  let timer: ReturnType<typeof setTimeout> | undefined

  const open = computed(() => b.value?.status === 'BORDEREAU_STATUS_OPEN')
  const executable = computed(() => open.value && (b.value?.pendingCount ?? 0) === 0)
  const building = computed(() => (b.value?.items ?? []).some(it => it.exportStatus === 'QUEUED' || it.exportStatus === 'RUNNING'))

  async function load () {
    clearTimeout(timer)
    loading.value = b.value === null
    try {
      b.value = (await getDispositionBordereau(id.value)) ?? null
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
    if (building.value) timer = setTimeout(load, POLL_MS)
  }
  watch(id, load, { immediate: true })
  onBeforeUnmount(() => clearTimeout(timer))

  async function execute () {
    busy.value = true
    try {
      b.value = (await executeDispositionBordereau(id.value)) ?? b.value
      ui.notify(t('retention.bordereau.executed'), 'success')
      await load()
    } catch (error) {
      report(error)
    } finally {
      busy.value = false
    }
  }

  async function cancel () {
    const validation = await cancelForm.value?.validate()
    if (validation && !validation.valid) return
    busy.value = true
    try {
      b.value = (await cancelDispositionBordereau(id.value, cancelReason.value.trim())) ?? b.value
      cancelling.value = false
      ui.notify(t('retention.bordereau.cancelled'), 'success')
    } catch (error) {
      report(error)
    } finally {
      busy.value = false
    }
  }

  // Saves the proof manifest byte for byte, so its SHA-256 can be recomputed.
  function saveManifest () {
    if (!b.value?.proofManifest) return
    const url = URL.createObjectURL(new Blob([b.value.proofManifest], { type: 'application/json' }))
    const a = document.createElement('a')
    a.href = url
    a.download = `bordereau-${b.value.id}-manifest.json`
    a.click()
    URL.revokeObjectURL(url)
  }

  async function download (item: DispositionItem) {
    if (!item.exportId) return
    downloading.value = item.id
    try {
      await downloadTransferPackage(item.exportId)
    } catch (error) {
      report(error)
    } finally {
      downloading.value = ''
    }
  }
</script>

<template>
  <v-container fluid>
    <v-alert v-if="!isRecordsManager" type="warning" variant="tonal">{{ t('retention.page.forbidden') }}</v-alert>
    <v-progress-linear v-else-if="loading" color="primary" indeterminate />

    <template v-else-if="b">
      <div class="d-flex flex-wrap align-center ga-2 mb-4">
        <v-btn icon="mdi-arrow-left" :title="t('retention.page.title')" to="/retention" variant="text" />
        <h1 class="text-h5">{{ t('retention.bordereau.title') }}</h1>
        <v-chip :color="dispositionColor(b.finalDisposition)" label size="small">{{ enumLabel('FinalDisposition', b.finalDisposition) }}</v-chip>
        <v-chip :color="bordereauStatusColor(b.status)" label size="small">{{ enumLabel('BordereauStatus', b.status) }}</v-chip>
      </div>

      <v-card class="mb-4">
        <v-card-text>
          <p><strong>{{ b.reason }}</strong></p>

          <p class="text-caption text-medium-emphasis">
            {{ t('retention.bordereau.preparedBy', { at: formatDateTime(b.preparedAt) }) }} <UserLabel :id="b.preparedBy" />
          </p>

          <p v-if="b.closedAt" class="text-caption text-medium-emphasis">
            {{ t('retention.bordereau.closedBy', { at: formatDateTime(b.closedAt) }) }} <UserLabel :id="b.closedBy" />
            <span v-if="b.closeReason"> — {{ b.closeReason }}</span>
          </p>

          <p v-if="b.proofSha256" class="text-caption">
            {{ t('retention.bordereau.proof') }} <code class="text-break">{{ b.proofSha256 }}</code>

            <v-btn
              v-if="b.proofManifest"
              class="ml-2"
              prepend-icon="mdi-download"
              size="x-small"
              variant="tonal"
              @click="saveManifest"
            >{{ t('retention.bordereau.manifest') }}</v-btn>
          </p>

          <p v-if="open" class="text-caption text-medium-emphasis mt-2">{{ t('retention.bordereau.approvalHint') }}</p>

          <div v-if="open" class="d-flex flex-wrap ga-2 mt-3">
            <v-btn
              color="primary"
              :disabled="!executable"
              :loading="busy"
              prepend-icon="mdi-check-all"
              variant="flat"
              @click="execute"
            >{{ t('retention.bordereau.execute') }}</v-btn>

            <v-btn prepend-icon="mdi-close-circle-outline" variant="tonal" @click="cancelling = true">{{ t('retention.bordereau.cancel') }}</v-btn>
          </div>

          <v-form v-if="cancelling" ref="cancelForm" class="mt-3" @submit.prevent="cancel">
            <v-text-field v-model="cancelReason" :label="t('retention.bordereau.cancelReason')" :rules="[required(t), maxLength(t, 2000)]" />
            <v-btn color="error" :loading="busy" type="submit">{{ t('retention.bordereau.cancel') }}</v-btn>
          </v-form>
        </v-card-text>
      </v-card>

      <v-card>
        <v-table density="compact">
          <thead>
            <tr>
              <th scope="col">{{ t('retention.page.case') }}</th>
              <th scope="col">{{ t('retention.page.until') }}</th>
              <th scope="col">{{ t('retention.bordereau.approver') }}</th>
              <th scope="col">{{ t('retention.bordereau.decision') }}</th>
              <th scope="col">{{ t('retention.bordereau.package') }}</th>
            </tr>
          </thead>

          <tbody>
            <tr v-for="item in b.items" :key="item.id">
              <td>
                <router-link :to="`/cases/${item.caseId}`">
                  <span v-if="item.businessRef" class="text-medium-emphasis mr-1">{{ item.businessRef }}</span>
                  {{ item.caseLabel || t('retention.page.hidden') }}
                </router-link>
              </td>

              <td>{{ formatDate(item.untilOn) }}</td>
              <td>{{ item.approverLabel || t('retention.page.hidden') }}</td>

              <td>
                <v-chip :color="decisionColor(item.decision)" label size="x-small">{{ enumLabel('DispositionDecision', item.decision) }}</v-chip>
                <div v-if="item.decidedBy" class="text-caption"><UserLabel :id="item.decidedBy" /> — {{ item.decisionReason }}</div>
              </td>

              <td>
                <v-btn
                  v-if="item.exportStatus === 'DONE'"
                  :loading="downloading === item.id"
                  prepend-icon="mdi-download"
                  size="small"
                  variant="text"
                  @click="download(item)"
                >{{ t('retention.bordereau.download') }}</v-btn>

                <span v-else-if="item.exportStatus" class="text-caption">{{ enumLabel('CaseExportStatus', `CASE_EXPORT_STATUS_${item.exportStatus}`) }}</span>
                <div v-if="item.exportSha256" class="text-caption text-medium-emphasis"><code>{{ item.exportSha256.slice(0, 16) }}…</code></div>
              </td>
            </tr>
          </tbody>
        </v-table>
      </v-card>
    </template>
  </v-container>
</template>
