<script setup lang="ts">
  import type { CaseExport } from '@/api/types'
  import { computed, onBeforeUnmount, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { downloadCaseExport, listCaseExports, requestCaseExport } from '@/api/exportClient'
  import UserLabel from '@/components/core/UserLabel.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { useAuthStore } from '@/stores/auth'
  import { useUiStore } from '@/stores/ui'
  import { formatBytes, formatDateTime } from '@/utils/formatters'
  import { maxLength } from '@/utils/validation'

  // The exports of a case (GLD-029): a holder of MANAGE asks for one, a worker
  // builds the ZIP in the background (the list refreshes until it is ready) and
  // its requester downloads it until it expires. Requests are recorded in the
  // sensitive read audit.
  const props = defineProps<{ caseId: string }>()
  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const { report } = useApiErrors()
  const auth = useAuthStore()
  const ui = useUiStore()

  const POLL_MS = 3000
  const STATUS_COLORS: Record<string, string> = {
    CASE_EXPORT_STATUS_QUEUED: 'info',
    CASE_EXPORT_STATUS_RUNNING: 'info',
    CASE_EXPORT_STATUS_DONE: 'success',
    CASE_EXPORT_STATUS_FAILED: 'error',
    CASE_EXPORT_STATUS_EXPIRED: 'default',
  }

  const exports = ref<CaseExport[]>([])
  const loading = ref(false)
  const asking = ref(false)
  const reason = ref('')
  const busy = ref(false)
  const downloading = ref('')
  let timer: ReturnType<typeof setTimeout> | undefined

  const pending = computed(() => exports.value.some(e => e.status === 'CASE_EXPORT_STATUS_QUEUED' || e.status === 'CASE_EXPORT_STATUS_RUNNING'))

  async function load () {
    clearTimeout(timer)
    loading.value = exports.value.length === 0
    try {
      exports.value = await listCaseExports(props.caseId)
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
    if (pending.value) {
      timer = setTimeout(load, POLL_MS)
    }
  }

  async function submit () {
    busy.value = true
    try {
      await requestCaseExport(props.caseId, reason.value.trim())
      ui.notify(t('export.requested'), 'success')
      asking.value = false
      reason.value = ''
      await load()
    } catch (error) {
      report(error)
    } finally {
      busy.value = false
    }
  }

  async function download (e: CaseExport) {
    downloading.value = e.id
    try {
      await downloadCaseExport(e.id)
    } catch (error) {
      report(error)
    } finally {
      downloading.value = ''
    }
  }

  /** Only the requester downloads, while the archive is built. */
  function downloadable (e: CaseExport) {
    return e.status === 'CASE_EXPORT_STATUS_DONE' && e.requestedBy === auth.me?.id
  }

  watch(() => props.caseId, load, { immediate: true })
  onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <div>
    <p class="text-caption text-medium-emphasis mb-2">{{ t('export.hint') }}</p>

    <v-form v-if="asking" class="mb-3" @submit.prevent="submit">
      <v-textarea
        v-model="reason"
        auto-grow
        counter="2000"
        :label="t('export.reason')"
        rows="2"
        :rules="[maxLength(t, 2000)]"
      />

      <div class="d-flex ga-2">
        <v-btn color="primary" :loading="busy" type="submit">{{ t('export.submit') }}</v-btn>
        <v-btn variant="text" @click="asking = false">{{ t('actions.common.cancel') }}</v-btn>
      </div>
    </v-form>

    <v-btn
      v-else
      class="mb-3"
      prepend-icon="mdi-folder-zip-outline"
      variant="tonal"
      @click="asking = true"
    >{{ t('export.ask') }}</v-btn>

    <v-progress-linear v-if="loading" class="mb-2" color="primary" indeterminate />
    <p v-else-if="exports.length === 0" class="text-medium-emphasis">{{ t('export.empty') }}</p>

    <v-table v-else density="compact">
      <thead>
        <tr>
          <th scope="col">{{ t('export.requestedAt') }}</th>
          <th scope="col">{{ t('export.requestedBy') }}</th>
          <th scope="col">{{ t('export.status') }}</th>
          <th scope="col">{{ t('export.archive') }}</th>
        </tr>
      </thead>

      <tbody>
        <tr v-for="e in exports" :key="e.id">
          <td class="text-caption">{{ formatDateTime(e.requestedAt) }}</td>
          <td><UserLabel :id="e.requestedBy" /></td>

          <td>
            <v-chip :color="STATUS_COLORS[e.status ?? ''] ?? 'default'" label size="x-small">{{ enumLabel('CaseExportStatus', e.status) }}</v-chip>
            <span v-if="e.error" class="text-caption text-error d-block">{{ e.error }}</span>
          </td>

          <td>
            <template v-if="e.status === 'CASE_EXPORT_STATUS_DONE'">
              <v-btn
                v-if="downloadable(e)"
                :loading="downloading === e.id"
                prepend-icon="mdi-download"
                size="small"
                variant="text"
                @click="download(e)"
              >{{ formatBytes(e.sizeBytes) }}</v-btn>

              <span v-else class="text-caption">{{ formatBytes(e.sizeBytes) }}</span>
              <span class="text-caption text-medium-emphasis d-block">{{ t('export.expires', { at: formatDateTime(e.expiresAt) }) }}</span>
            </template>
          </td>
        </tr>
      </tbody>
    </v-table>
  </div>
</template>
