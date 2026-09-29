<script setup lang="ts">
  import type { TimelineEntry } from '@/api/types'
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import UserLabel from '@/components/core/UserLabel.vue'
  import { entryStatusColor, entryTypeStyle, isCorrectable, isDraft, metadataText } from '@/components/timeline/timelineForm'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { formatDateTime } from '@/utils/formatters'

  // One timeline entry: type, status, business date, author, content, cited
  // documents (with the pinned version once frozen) and correction links.
  const props = defineProps<{ entry: TimelineEntry, canEdit?: boolean }>()
  const emit = defineEmits<{
    edit: [entry: TimelineEntry]
    status: [entry: TimelineEntry, action: 'validate' | 'lock' | 'withdraw']
    correct: [entry: TimelineEntry]
  }>()

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()

  const style = computed(() => entryTypeStyle(props.entry.entryType))
  const draft = computed(() => isDraft(props.entry))
  const withdrawn = computed(() => props.entry.status === 'TIMELINE_ENTRY_STATUS_WITHDRAWN')
  const isSystem = computed(() => props.entry.entryType === 'TIMELINE_ENTRY_TYPE_SYSTEM')
  const restricted = computed(() => props.entry.visibility && props.entry.visibility !== 'TIMELINE_VISIBILITY_CASE_PARTICIPANTS')

  // A case status change is rendered in the UI language from its structured
  // metadata; the stored French body is the fallback.
  const bodyText = computed(() => {
    const meta = props.entry.metadata
    if (!isSystem.value || metadataText(meta, 'event') !== 'CASE_STATUS_CHANGED') {
      return props.entry.body
    }
    const from = enumLabel('CaseStatus', `CASE_STATUS_${metadataText(meta, 'from')}`)
    const to = enumLabel('CaseStatus', `CASE_STATUS_${metadataText(meta, 'to')}`)
    const reason = metadataText(meta, 'reason')
    const text = t('timeline.system.statusChanged', { from, to })
    return reason ? `${text}\n${t('timeline.system.reason', { reason })}` : text
  })
  const heading = computed(() => (isSystem.value ? t('timeline.system.title') : props.entry.title))

  const frozenBy = computed(() => props.entry.validatedBy || props.entry.lockedBy)
  const frozenAt = computed(() => props.entry.validatedAt || props.entry.lockedAt)
</script>

<template>
  <v-card :id="`entry-${entry.id}`" :class="{ 'entry-withdrawn': withdrawn }" variant="outlined">
    <v-card-item>
      <div class="d-flex flex-wrap align-center ga-2">
        <span class="text-subtitle-2" :class="`text-${style.color}`">{{ enumLabel('TimelineEntryType', entry.entryType) }}</span>
        <v-chip v-if="!isSystem" :color="entryStatusColor(entry)" label size="x-small">{{ enumLabel('TimelineEntryStatus', entry.status) }}</v-chip>

        <v-chip v-if="restricted" prepend-icon="mdi-eye-lock-outline" size="x-small" variant="outlined">
          {{ enumLabel('TimelineVisibility', entry.visibility) }}
        </v-chip>

        <v-spacer />

        <span class="text-caption text-medium-emphasis" :title="t('timeline.fields.recordedAt', { date: formatDateTime(entry.createdAt) })">
          {{ formatDateTime(entry.occurredAt) }} · <UserLabel :id="entry.createdBy" />
        </span>
      </div>

      <div v-if="heading" class="text-subtitle-1 mt-1">{{ heading }}</div>
    </v-card-item>

    <v-card-text>
      <p class="entry-body">{{ bodyText }}</p>

      <div v-if="entry.documents?.length" class="d-flex flex-wrap ga-2 mt-3">
        <v-chip
          v-for="doc in entry.documents"
          :key="doc.id"
          prepend-icon="mdi-file-document-outline"
          size="small"
          :to="`/documents/${encodeURIComponent(doc.documentId)}`"
        >
          {{ doc.documentLabel ?? doc.documentId }}
          <span v-if="doc.documentVersionNo" class="ml-1 text-medium-emphasis">
            · {{ t('timeline.pinnedVersion', { n: doc.documentVersionNo }) }}
          </span>
        </v-chip>
      </div>

      <div class="text-caption text-medium-emphasis mt-3 d-flex flex-column ga-1">
        <span v-if="frozenAt">
          {{ t(entry.validatedAt ? 'timeline.validatedBy' : 'timeline.lockedBy', { date: formatDateTime(frozenAt) }) }}
          <UserLabel :id="frozenBy" />
        </span>

        <span v-if="entry.updatedAt && draft">{{ t('timeline.updatedAt', { date: formatDateTime(entry.updatedAt) }) }}</span>

        <span v-if="withdrawn">
          {{ t('timeline.withdrawnBy', { date: formatDateTime(entry.withdrawnAt), reason: entry.withdrawalReason }) }}
        </span>

        <a v-if="entry.correctsEntryId" class="entry-anchor" :href="`#entry-${entry.correctsEntryId}`">
          <v-icon icon="mdi-arrow-u-left-top" size="x-small" /> {{ t('timeline.correctsLink') }}
        </a>

        <a v-if="entry.correctedByEntryId" class="entry-anchor" :href="`#entry-${entry.correctedByEntryId}`">
          <v-icon icon="mdi-alert-circle-outline" size="x-small" /> {{ t('timeline.correctedByLink') }}
        </a>
      </div>
    </v-card-text>

    <v-card-actions v-if="canEdit && (draft || isCorrectable(entry))">
      <template v-if="draft">
        <v-btn prepend-icon="mdi-pencil" size="small" variant="text" @click="emit('edit', entry)">{{ t('timeline.actions.edit') }}</v-btn>

        <v-btn
          color="success"
          prepend-icon="mdi-check-decagram"
          size="small"
          variant="tonal"
          @click="emit('status', entry, 'validate')"
        >
          {{ t('timeline.actions.validate') }}
        </v-btn>

        <v-btn prepend-icon="mdi-lock-outline" size="small" variant="text" @click="emit('status', entry, 'lock')">{{ t('timeline.actions.lock') }}</v-btn>
        <v-spacer />

        <v-btn
          color="error"
          prepend-icon="mdi-archive-arrow-down-outline"
          size="small"
          variant="text"
          @click="emit('status', entry, 'withdraw')"
        >
          {{ t('timeline.actions.withdraw') }}
        </v-btn>
      </template>

      <v-btn
        v-else
        prepend-icon="mdi-file-replace-outline"
        size="small"
        variant="text"
        @click="emit('correct', entry)"
      >
        {{ t('timeline.actions.correct') }}
      </v-btn>
    </v-card-actions>
  </v-card>
</template>

<style scoped>
  .entry-body {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .entry-withdrawn {
    opacity: 0.6;
  }

  .entry-anchor {
    color: rgb(var(--v-theme-primary));
    text-decoration: none;
  }

  .entry-anchor:hover,
  .entry-anchor:focus-visible {
    text-decoration: underline;
  }
</style>
