<script setup lang="ts">
  import type { Circulation, CirculationRecipient } from '@/api/types'
  import { ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { cancelCirculation, listCaseCirculations } from '@/api/circulationClient'
  import CirculationCreateDialog from '@/components/circulation/CirculationCreateDialog.vue'
  import { byStep, circulationStatusColor, responseColor } from '@/components/circulation/circulationForm'
  import CirculationRespondDialog from '@/components/circulation/CirculationRespondDialog.vue'
  import UserLabel from '@/components/core/UserLabel.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { useUiStore } from '@/stores/ui'
  import { formatDate, formatDateTime } from '@/utils/formatters'
  import { required } from '@/utils/validation'

  // The circulations of a case: recipients by step with their answer, the
  // awaited ones answerable here. Reports the open count (a case cannot close
  // with an open circulation) and asks the page to reload after a change.
  const props = defineProps<{ caseId: string, canEdit?: boolean, reloadKey?: number }>()
  const emit = defineEmits<{ changed: [], open: [count: number] }>()

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const circulations = ref<Circulation[]>([])
  const loading = ref(false)
  const createOpen = ref(false)
  const respondOpen = ref(false)
  const respondTo = ref<CirculationRecipient | undefined>()
  const respondTitle = ref('')
  const cancelling = ref<Circulation | null>(null)
  const cancelReason = ref('')
  const cancelBusy = ref(false)

  async function load () {
    loading.value = true
    try {
      circulations.value = await listCaseCirculations(props.caseId)
      emit('open', circulations.value.filter(c => c.status === 'CIRCULATION_STATUS_OPEN').length)
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  async function changed () {
    await load()
    emit('changed')
  }

  function askRespond (c: Circulation, rec: CirculationRecipient) {
    respondTo.value = rec
    respondTitle.value = c.title
    respondOpen.value = true
  }

  function askCancel (c: Circulation) {
    cancelling.value = c
    cancelReason.value = ''
  }

  async function doCancel () {
    const c = cancelling.value
    if (!c || !cancelReason.value.trim()) return
    cancelBusy.value = true
    try {
      await cancelCirculation(c.id, cancelReason.value.trim())
      ui.notify(t('circulations.messages.cancelled'), 'success')
      cancelling.value = null
      await changed()
    } catch (error) {
      report(error)
    } finally {
      cancelBusy.value = false
    }
  }

  watch([() => props.caseId, () => props.reloadKey], load, { immediate: true })
</script>

<template>
  <div>
    <div class="d-flex align-center mb-2">
      <v-spacer />

      <v-btn
        v-if="canEdit"
        color="primary"
        prepend-icon="mdi-send-outline"
        size="small"
        variant="flat"
        @click="createOpen = true"
      >
        {{ t('circulations.actions.create') }}
      </v-btn>
    </div>

    <v-progress-linear v-if="loading" class="mb-2" color="primary" indeterminate />
    <p v-if="!loading && circulations.length === 0" class="text-medium-emphasis">{{ t('circulations.empty') }}</p>

    <v-card v-for="c in circulations" :key="c.id" class="mb-3" variant="outlined">
      <v-card-item>
        <div class="d-flex flex-wrap align-center ga-2">
          <span class="text-subtitle-1">{{ c.title }}</span>
          <v-chip :color="circulationStatusColor(c.status)" label size="x-small">{{ enumLabel('CirculationStatus', c.status) }}</v-chip>

          <v-chip v-if="c.status === 'CIRCULATION_STATUS_OPEN' && (c.stepCount ?? 1) > 1" size="x-small" variant="outlined">
            {{ t('circulations.stepOf', { step: c.currentStep, count: c.stepCount }) }}
          </v-chip>

          <v-chip v-if="c.overdue" color="error" prepend-icon="mdi-alarm" size="x-small">{{ t('circulations.overdue', { date: formatDate(c.dueAt) }) }}</v-chip>
          <span v-else-if="c.dueAt" class="text-caption text-medium-emphasis">{{ t('circulations.due', { date: formatDate(c.dueAt) }) }}</span>
          <v-spacer />
          <span class="text-caption text-medium-emphasis">{{ formatDateTime(c.createdAt) }} · <UserLabel :id="c.createdBy" /></span>
        </div>

        <p v-if="c.message" class="text-body-2 mt-1 circulation-message">{{ c.message }}</p>
      </v-card-item>

      <v-card-text>
        <div v-for="group in byStep(c.recipients ?? [])" :key="group.step" class="mb-2">
          <div v-if="(c.stepCount ?? 1) > 1" class="text-overline text-medium-emphasis">{{ t('circulations.step', { step: group.step }) }}</div>

          <div v-for="rec in group.recipients" :key="rec.id" class="d-flex flex-wrap align-center ga-2 py-1">
            <v-icon :icon="rec.assigneeOrgUnitId ? 'mdi-sitemap-outline' : 'mdi-account-circle-outline'" size="small" />
            <span class="font-weight-medium">{{ rec.assigneeLabel }}</span>

            <v-chip v-if="rec.response && rec.response !== 'CIRCULATION_RESPONSE_UNSPECIFIED'" :color="responseColor(rec.response)" label size="x-small">
              {{ enumLabel('CirculationResponse', rec.response) }}
            </v-chip>

            <v-chip v-else-if="rec.awaiting" color="warning" label size="x-small">{{ t('circulations.awaiting') }}</v-chip>
            <span v-else class="text-caption text-medium-emphasis">{{ t('circulations.notYet') }}</span>

            <span v-if="rec.responseText" class="text-body-2 text-medium-emphasis">« {{ rec.responseText }} »</span>
            <v-spacer />

            <v-btn
              v-if="canEdit && rec.awaiting"
              prepend-icon="mdi-reply-outline"
              size="small"
              variant="tonal"
              @click="askRespond(c, rec)"
            >
              {{ t('circulations.actions.respond') }}
            </v-btn>
          </div>
        </div>

        <p v-if="c.cancellationReason" class="text-caption text-medium-emphasis">{{ t('circulations.cancelledBecause', { reason: c.cancellationReason }) }}</p>
      </v-card-text>

      <v-card-actions v-if="canEdit && c.status === 'CIRCULATION_STATUS_OPEN'">
        <v-spacer />

        <v-btn
          color="error"
          prepend-icon="mdi-close-circle-outline"
          size="small"
          variant="text"
          @click="askCancel(c)"
        >{{ t('circulations.actions.cancel') }}</v-btn>
      </v-card-actions>
    </v-card>

    <CirculationCreateDialog v-model="createOpen" :case-id="caseId" @created="changed" />
    <CirculationRespondDialog v-model="respondOpen" :circulation-title="respondTitle" :recipient="respondTo" @responded="changed" />

    <v-dialog max-width="480" :model-value="!!cancelling" @update:model-value="cancelling = null">
      <v-card>
        <v-card-title>{{ t('circulations.actions.cancel') }}</v-card-title>

        <v-card-text>
          <p class="mb-3">{{ t('circulations.cancelConfirm') }}</p>
          <v-text-field v-model="cancelReason" :label="t('finalize.reason')" :rules="[required(t)]" />
        </v-card-text>

        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="cancelling = null">{{ t('actions.common.cancel') }}</v-btn>
          <v-btn color="error" :loading="cancelBusy" variant="flat" @click="doCancel">{{ t('actions.common.confirm') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<style scoped>
  .circulation-message {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
</style>
