<script setup lang="ts">
  import type { AuditEvent } from '@/api/types'
  import { useI18n } from 'vue-i18n'
  import { formatDateTime } from '@/utils/formatters'

  // AuditEvents are append-only: this component is strictly read-only and never
  // offers edit/delete affordances.
  defineProps<{ events?: AuditEvent[] }>()
  const { t } = useI18n()

  function hasState (e: AuditEvent): boolean {
    return !!(e.beforeState || e.afterState)
  }
</script>

<template>
  <div>
    <p v-if="!events || events.length === 0" class="text-medium-emphasis">
      {{ t('messages.common.noData') }}
    </p>

    <v-timeline v-else density="compact" side="end" truncate-line="both">
      <v-timeline-item
        v-for="(event, i) in events"
        :key="event.id ?? i"
        dot-color="primary"
        size="x-small"
      >
        <div class="d-flex justify-space-between flex-wrap ga-2">
          <strong>{{ event.eventType }}</strong>
          <span class="text-caption text-medium-emphasis">{{ formatDateTime(event.occurredAt) }}</span>
        </div>

        <div v-if="event.actorUserId" class="text-caption">
          {{ t('fields.audit.actor_user_id') }}: {{ event.actorUserId }}
        </div>

        <div v-if="event.reason" class="text-body-2">{{ event.reason }}</div>

        <v-expansion-panels v-if="hasState(event)" class="mt-1" variant="accordion">
          <v-expansion-panel :title="t('fields.audit.before_state') + ' / ' + t('fields.audit.after_state')">
            <template #text>
              <pre class="audit-json">{{ JSON.stringify({ before: event.beforeState, after: event.afterState }, null, 2) }}</pre>
            </template>
          </v-expansion-panel>
        </v-expansion-panels>
      </v-timeline-item>
    </v-timeline>
  </div>
</template>

<style scoped>
.audit-json {
  font-size: 0.75rem;
  white-space: pre-wrap;
  word-break: break-word;
  overflow-x: auto;
}
</style>
