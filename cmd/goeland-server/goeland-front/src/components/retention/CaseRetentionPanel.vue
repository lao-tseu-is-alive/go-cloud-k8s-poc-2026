<script setup lang="ts">
  import type { GetCaseRetentionResponse } from '@/api/types'
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { getCaseRetention, placeRetentionHold, releaseRetentionHold } from '@/api/retentionClient'
  import UserLabel from '@/components/core/UserLabel.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { useAuthStore } from '@/stores/auth'
  import { useUiStore } from '@/stores/ui'
  import { formatDate, formatDateTime } from '@/utils/formatters'
  import { dispositionColor, isDue, zurichToday } from '@/utils/retention'
  import { maxLength, required } from '@/utils/validation'

  // The retention of a case (GLD-032): the running retention (a snapshot of its
  // type's policy taken at closing) or the policy that will apply, the legal
  // hold the records manager places or releases, and the history.
  const props = defineProps<{ caseId: string, closed: boolean }>()
  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const { report } = useApiErrors()
  const auth = useAuthStore()
  const ui = useUiStore()

  const data = ref<GetCaseRetentionResponse>({})
  const loading = ref(false)
  const editing = ref<'place' | 'release' | null>(null)
  const reason = ref('')
  const busy = ref(false)
  const form = ref()

  const current = computed(() => data.value.current)
  const due = computed(() => isDue(current.value?.untilOn, zurichToday()))
  const pastRetentions = computed(() => (data.value.retentions ?? []).filter(r => r.endedAt))
  const pastHolds = computed(() => (data.value.holds ?? []).filter(h => h.releasedAt))

  async function load () {
    loading.value = true
    try {
      data.value = await getCaseRetention(props.caseId)
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  function open (mode: 'place' | 'release') {
    editing.value = mode
    reason.value = ''
  }

  async function submit () {
    const validation = await form.value?.validate()
    if (validation && !validation.valid) return
    busy.value = true
    try {
      if (editing.value === 'place') {
        await placeRetentionHold(props.caseId, reason.value.trim())
        ui.notify(t('retention.holdPlaced'), 'success')
      } else {
        await releaseRetentionHold(props.caseId, reason.value.trim())
        ui.notify(t('retention.holdReleased'), 'success')
      }
      editing.value = null
      await load()
    } catch (error) {
      report(error)
    } finally {
      busy.value = false
    }
  }

  watch(() => [props.caseId, props.closed], load, { immediate: true })
</script>

<template>
  <div>
    <p class="text-caption text-medium-emphasis mb-2">{{ t('retention.hint') }}</p>
    <v-progress-linear v-if="loading" class="mb-2" color="primary" indeterminate />

    <div v-if="current" class="mb-2">
      <div class="d-flex flex-wrap align-center ga-2">
        <strong>{{ t('retention.running', { until: formatDate(current.untilOn) }) }}</strong>
        <v-chip :color="dispositionColor(current.finalDisposition)" label size="small">{{ enumLabel('FinalDisposition', current.finalDisposition) }}</v-chip>

        <v-chip
          v-if="current.dispositionStatus === 'DISPOSITION_STATUS_ARCHIVED'"
          color="primary"
          label
          prepend-icon="mdi-archive-check"
          size="small"
        >
          {{ t('retention.archived', { at: formatDate(current.disposedAt) }) }}
        </v-chip>

        <v-chip v-else-if="due" color="warning" label size="small">{{ t('retention.due') }}</v-chip>
      </div>

      <div class="text-caption text-medium-emphasis">
        {{ t('retention.since', { start: formatDate(current.startsOn), years: current.durationYears ?? 0, policy: current.policyLabel || current.policyCode }) }}
      </div>
    </div>

    <p v-else-if="!loading && !closed && data.defaultPolicy" class="mb-2">{{ t('retention.notClosed', { policy: data.defaultPolicy.label }) }}</p>
    <p v-else-if="!loading && closed" class="mb-2 text-medium-emphasis">{{ t('retention.closedWithout') }}</p>
    <p v-else-if="!loading" class="mb-2 text-medium-emphasis">{{ t('retention.noPolicy') }}</p>

    <v-alert
      v-if="data.hold"
      class="mb-2"
      density="compact"
      icon="mdi-lock-outline"
      type="warning"
      variant="tonal"
    >
      <strong>{{ t('retention.onHold') }}</strong> — {{ t('retention.holdSince', { at: formatDateTime(data.hold.placedAt), reason: data.hold.reason }) }}
      <div class="text-caption"><UserLabel :id="data.hold.placedBy" /></div>
    </v-alert>

    <template v-if="auth.isRecordsManager">
      <v-form v-if="editing" ref="form" class="mb-2" @submit.prevent="submit">
        <v-textarea
          v-model="reason"
          auto-grow
          counter="2000"
          :label="editing === 'place' ? t('retention.holdReason') : t('retention.releaseReason')"
          rows="2"
          :rules="[required(t), maxLength(t, 2000)]"
        />

        <div class="d-flex ga-2">
          <v-btn color="primary" :loading="busy" type="submit">{{ editing === 'place' ? t('retention.placeHold') : t('retention.releaseHold') }}</v-btn>
          <v-btn variant="text" @click="editing = null">{{ t('actions.common.cancel') }}</v-btn>
        </div>
      </v-form>

      <v-btn
        v-else-if="data.hold"
        class="mb-2"
        prepend-icon="mdi-lock-open-variant-outline"
        variant="tonal"
        @click="open('release')"
      >{{ t('retention.releaseHold') }}</v-btn>

      <v-btn
        v-else
        class="mb-2"
        prepend-icon="mdi-lock-outline"
        variant="tonal"
        @click="open('place')"
      >{{ t('retention.placeHold') }}</v-btn>
    </template>

    <v-expansion-panels v-if="pastRetentions.length > 0 || pastHolds.length > 0" variant="accordion">
      <v-expansion-panel :title="t('retention.history')">
        <v-expansion-panel-text>
          <ul class="text-caption">
            <li v-for="r in pastRetentions" :key="r.id">
              {{ formatDate(r.startsOn) }} → {{ formatDate(r.untilOn) }}, {{ r.policyLabel || r.policyCode }} —
              {{ t('retention.ended', { at: formatDateTime(r.endedAt), reason: r.endReason || '—' }) }}
            </li>

            <li v-for="h in pastHolds" :key="h.id">
              {{ t('retention.holdSince', { at: formatDateTime(h.placedAt), reason: h.reason }) }} —
              {{ t('retention.released', { at: formatDateTime(h.releasedAt), reason: h.releaseReason || '—' }) }}
            </li>
          </ul>
        </v-expansion-panel-text>
      </v-expansion-panel>
    </v-expansion-panels>
  </div>
</template>
