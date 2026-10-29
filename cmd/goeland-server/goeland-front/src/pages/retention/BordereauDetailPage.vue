<script setup lang="ts">
  import type { DispositionBordereau, DispositionItem, EliminationPreview } from '@/api/types'
  import { storeToRefs } from 'pinia'
  import { computed, onBeforeUnmount, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute } from 'vue-router'
  import { cancelDispositionBordereau, downloadTransferPackage, executeDispositionBordereau, getDispositionBordereau, rebuildTransferPackage, simulateElimination } from '@/api/retentionClient'
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
  const { disposalEnabled, isRecordsManager } = storeToRefs(useAuthStore())
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
  // An elimination is irreversible: its execution asks for an explicit confirmation.
  const destroying = computed(() => b.value?.finalDisposition === 'FINAL_DISPOSITION_DESTROY')
  const confirmOpen = ref(false)
  const confirmed = ref(false)
  // Off on this server (GOELAND_DISPOSAL_ENABLED), an elimination is only simulated.
  const canExecute = computed(() => !destroying.value || disposalEnabled.value)
  const simulating = ref(false)
  const preview = ref<EliminationPreview[] | null>(null)
  const rebuilding = ref('')
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

  function askExecute () {
    if (destroying.value) {
      confirmed.value = false
      confirmOpen.value = true
      return
    }
    void execute()
  }

  // Runs the elimination and rolls it back: what each approved case would lose.
  async function simulate () {
    simulating.value = true
    try {
      preview.value = await simulateElimination(id.value)
    } catch (error) {
      report(error)
    } finally {
      simulating.value = false
    }
  }

  function itemOf (caseId?: string): DispositionItem | undefined {
    return b.value?.items?.find(it => it.caseId === caseId)
  }

  // A failed or expired transfer package is queued again.
  async function rebuild (item: DispositionItem) {
    rebuilding.value = item.id
    try {
      await rebuildTransferPackage(item.id)
      ui.notify(t('retention.bordereau.rebuilt'), 'success')
      await load()
    } catch (error) {
      report(error)
    } finally {
      rebuilding.value = ''
    }
  }

  async function execute () {
    confirmOpen.value = false
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

          <v-alert
            v-if="open && !canExecute"
            class="mt-3"
            density="compact"
            type="info"
            variant="tonal"
          >{{ t('retention.bordereau.disposalDisabled') }}</v-alert>

          <div v-if="open" class="d-flex flex-wrap ga-2 mt-3">
            <v-btn
              v-if="destroying"
              :disabled="!executable"
              :loading="simulating"
              prepend-icon="mdi-play-outline"
              variant="tonal"
              @click="simulate"
            >{{ t('retention.bordereau.simulate') }}</v-btn>

            <v-btn
              v-if="canExecute"
              :color="destroying ? 'error' : 'primary'"
              :disabled="!executable"
              :loading="busy"
              :prepend-icon="destroying ? 'mdi-delete-forever' : 'mdi-check-all'"
              variant="flat"
              @click="askExecute"
            >{{ destroying ? t('retention.bordereau.eliminate') : t('retention.bordereau.execute') }}</v-btn>

            <v-btn prepend-icon="mdi-close-circle-outline" variant="tonal" @click="cancelling = true">{{ t('retention.bordereau.cancel') }}</v-btn>
          </div>

          <v-form v-if="cancelling" ref="cancelForm" class="mt-3" @submit.prevent="cancel">
            <v-text-field v-model="cancelReason" :label="t('retention.bordereau.cancelReason')" :rules="[required(t), maxLength(t, 2000)]" />
            <v-btn color="error" :loading="busy" type="submit">{{ t('retention.bordereau.cancel') }}</v-btn>
          </v-form>
        </v-card-text>
      </v-card>

      <v-card v-if="preview" class="mb-4">
        <v-card-title class="text-subtitle-1">{{ t('retention.bordereau.previewTitle') }}</v-card-title>
        <v-card-subtitle class="text-wrap">{{ t('retention.bordereau.previewHint') }}</v-card-subtitle>

        <v-table density="compact">
          <thead>
            <tr>
              <th scope="col">{{ t('retention.page.case') }}</th>
              <th scope="col">{{ t('retention.bordereau.documentsEliminated') }}</th>
              <th scope="col">{{ t('retention.bordereau.documentsKept') }}</th>
              <th scope="col">{{ t('retention.bordereau.filesDestroyed') }}</th>
              <th scope="col">{{ t('retention.bordereau.exportsPurged') }}</th>
            </tr>
          </thead>

          <tbody>
            <tr v-for="p in preview" :key="p.caseId">
              <td>
                <span v-if="itemOf(p.caseId)?.businessRef" class="text-medium-emphasis mr-1">{{ itemOf(p.caseId)?.businessRef }}</span>
                {{ itemOf(p.caseId)?.caseLabel || t('retention.page.hidden') }}
              </td>

              <td>{{ p.documentsEliminated ?? 0 }}</td>
              <td>{{ p.documentsKept ?? 0 }}</td>
              <td>{{ p.filesDestroyed ?? 0 }}</td>
              <td>{{ p.exportsPurged ?? 0 }}</td>
            </tr>
          </tbody>
        </v-table>
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

                <v-btn
                  v-else-if="item.exportStatus === 'FAILED' || item.exportStatus === 'EXPIRED'"
                  :loading="rebuilding === item.id"
                  prepend-icon="mdi-refresh"
                  size="small"
                  variant="text"
                  @click="rebuild(item)"
                >{{ t('retention.bordereau.rebuild') }}</v-btn>

                <span v-else-if="item.exportStatus" class="text-caption">{{ enumLabel('CaseExportStatus', `CASE_EXPORT_STATUS_${item.exportStatus}`) }}</span>
                <div v-if="item.exportSha256" class="text-caption text-medium-emphasis"><code>{{ item.exportSha256.slice(0, 16) }}…</code></div>
              </td>
            </tr>
          </tbody>
        </v-table>
      </v-card>

      <v-dialog v-model="confirmOpen" max-width="560">
        <v-card>
          <v-card-title>{{ t('retention.bordereau.eliminate') }}</v-card-title>

          <v-card-text>
            <v-alert class="mb-3" type="error" variant="tonal">{{ t('retention.bordereau.eliminateWarning', { n: b.approvedCount ?? 0 }) }}</v-alert>
            <v-checkbox v-model="confirmed" density="compact" hide-details :label="t('retention.bordereau.eliminateConfirm')" />
          </v-card-text>

          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="confirmOpen = false">{{ t('actions.common.cancel') }}</v-btn>

            <v-btn
              color="error"
              :disabled="!confirmed"
              :loading="busy"
              variant="flat"
              @click="execute"
            >{{ t('retention.bordereau.eliminate') }}</v-btn>
          </v-card-actions>
        </v-card>
      </v-dialog>
    </template>
  </v-container>
</template>
