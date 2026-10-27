<script setup lang="ts">
  import type { OutboxEvent, OutboxStatus } from '@/api/types'
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { getOutboxStatus, replayOutboxEvents } from '@/api/outboxClient'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useUiStore } from '@/stores/ui'
  import { formatDateTime } from '@/utils/formatters'
  import { kindIcon, subjectRoute } from '@/utils/subjects'

  // The transactional outbox (GLD-028): what waits for delivery, what was
  // given up and why, per consumer, and the replay of the given-up events.
  const { t } = useI18n()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const status = ref<OutboxStatus>({})
  const loading = ref(false)
  const replaying = ref('')
  const failed = computed(() => status.value.recentFailed ?? [])

  async function load () {
    loading.value = true
    try {
      status.value = await getOutboxStatus()
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  /** Replays one event, or every given-up event when ev is undefined. */
  async function replay (ev?: OutboxEvent) {
    replaying.value = ev?.id ?? 'all'
    try {
      const n = await replayOutboxEvents(ev ? { eventIds: [ev.id] } : { allFailed: true })
      ui.notify(t('outbox.replayed', n), 'success')
      await load()
    } catch (error) {
      report(error)
    } finally {
      replaying.value = ''
    }
  }

  function subjectLink (ev: OutboxEvent): string | undefined {
    return ev.subjectId && ev.subjectKind ? subjectRoute({ id: ev.subjectId, kind: ev.subjectKind, displayLabel: '' }) : undefined
  }

  onMounted(load)
</script>

<template>
  <div>
    <p class="text-body-2 text-medium-emphasis mb-4">{{ t('outbox.intro', { days: status.retentionDays ?? 7, attempts: status.maxAttempts ?? 10 }) }}</p>

    <div class="d-flex flex-wrap align-center ga-3 mb-4">
      <v-chip :color="status.pending ? 'primary' : undefined" label prepend-icon="mdi-tray-full">
        {{ t('outbox.pending') }} : {{ status.pending ?? 0 }}
      </v-chip>

      <v-chip :color="status.failed ? 'error' : undefined" label prepend-icon="mdi-alert-circle-outline">
        {{ t('outbox.failed') }} : {{ status.failed ?? 0 }}
      </v-chip>

      <v-chip label prepend-icon="mdi-check-all">{{ t('outbox.published') }} : {{ status.published ?? 0 }}</v-chip>

      <span v-if="status.oldestPendingAt" class="text-caption text-medium-emphasis">
        {{ t('outbox.oldestPending', { at: formatDateTime(status.oldestPendingAt) }) }}
      </span>

      <v-spacer />
      <v-btn :loading="loading" prepend-icon="mdi-refresh" variant="text" @click="load">{{ t('actions.common.refresh') }}</v-btn>
    </div>

    <h2 class="text-subtitle-1 mb-2">{{ t('outbox.consumers') }}</h2>
    <p v-if="!status.consumers?.length" class="text-body-2 text-medium-emphasis mb-4">{{ t('outbox.noConsumer') }}</p>

    <div v-else class="d-flex flex-wrap ga-2 mb-4">
      <v-chip
        v-for="c in status.consumers"
        :key="c.name"
        :color="c.failed ? 'error' : 'success'"
        label
        variant="tonal"
      >
        {{ c.name }}<span v-if="c.failed" class="ms-1">· {{ t('outbox.consumerFailed', c.failed) }}</span>
      </v-chip>
    </div>

    <div class="d-flex align-center mb-2">
      <h2 class="text-subtitle-1">{{ t('outbox.recentFailed') }}</h2>
      <v-spacer />

      <v-btn
        v-if="failed.length > 0"
        color="primary"
        :loading="replaying === 'all'"
        prepend-icon="mdi-replay"
        variant="tonal"
        @click="replay()"
      >{{ t('outbox.replayAll', { count: status.failed ?? 0 }) }}</v-btn>
    </div>

    <p v-if="failed.length === 0" class="text-body-2 text-medium-emphasis">{{ t('outbox.noFailure') }}</p>

    <v-table v-else density="compact">
      <thead>
        <tr>
          <th scope="col">{{ t('outbox.event') }}</th>
          <th scope="col">{{ t('outbox.subject') }}</th>
          <th scope="col">{{ t('outbox.attempts') }}</th>
          <th scope="col">{{ t('outbox.error') }}</th>
          <th scope="col">{{ t('outbox.failedAt') }}</th>
          <th scope="col"><span class="sr-only">{{ t('outbox.replay') }}</span></th>
        </tr>
      </thead>

      <tbody>
        <tr v-for="ev in failed" :key="ev.id">
          <td class="text-no-wrap">{{ ev.eventType }}</td>

          <td>
            <router-link v-if="subjectLink(ev)" :to="subjectLink(ev)!">
              <v-icon :icon="kindIcon(ev.subjectKind)" size="small" /> {{ ev.subjectId?.slice(0, 8) }}…
            </router-link>

            <span v-else class="text-medium-emphasis">{{ ev.subjectId?.slice(0, 8) }}…</span>
          </td>

          <td>{{ ev.attempts }}</td>
          <td class="text-caption outbox-panel__error">{{ ev.lastError }}</td>
          <td class="text-caption text-no-wrap">{{ formatDateTime(ev.failedAt) }}</td>

          <td>
            <v-btn
              :aria-label="t('outbox.replay')"
              icon="mdi-replay"
              :loading="replaying === ev.id"
              size="small"
              :title="t('outbox.replay')"
              variant="text"
              @click="replay(ev)"
            />
          </td>
        </tr>
      </tbody>
    </v-table>
  </div>
</template>

<style scoped>
  .outbox-panel__error {
    max-width: 420px;
    overflow-wrap: anywhere;
  }
</style>
