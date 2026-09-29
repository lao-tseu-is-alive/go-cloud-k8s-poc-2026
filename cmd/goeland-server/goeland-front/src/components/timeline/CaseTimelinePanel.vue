<script setup lang="ts">
  import type { TimelineEntry, TimelineEntryType } from '@/api/types'
  import type { EntrySubmission } from '@/components/timeline/timelineForm'
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import {
    changeTimelineEntryStatus,
    createTimelineEntry,
    linkTimelineDocument,
    listTimeline,
    unlinkTimelineDocument,
    updateTimelineEntry,
  } from '@/api/timelineClient'
  import TimelineEntryCard from '@/components/timeline/TimelineEntryCard.vue'
  import TimelineEntryDialog from '@/components/timeline/TimelineEntryDialog.vue'
  import { entryTypeStyle, FILTER_ENTRY_TYPES } from '@/components/timeline/timelineForm'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { useUiStore } from '@/stores/ui'
  import { required } from '@/utils/validation'

  // The case timeline ("suivis"): most recent business date first, filterable by
  // type, with draft editing, validation / locking / withdrawal and corrections.
  // reloadKey lets the page refresh it (a status change adds a SYSTEM entry);
  // changed tells the page to reload (citing a document links it to the case).
  const props = defineProps<{ caseId: string, canEdit?: boolean, reloadKey?: number }>()
  const emit = defineEmits<{ changed: [], drafts: [count: number] }>()

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const { report } = useApiErrors()
  const ui = useUiStore()
  const PAGE_SIZE = 25

  const entries = ref<TimelineEntry[]>([])
  const total = ref(0)
  const nextToken = ref('')
  const loading = ref(false)
  const types = ref<TimelineEntryType[]>([])
  const includeWithdrawn = ref(false)

  const dialogOpen = ref(false)
  const dialogMode = ref<'create' | 'edit' | 'correct'>('create')
  const dialogEntry = ref<TimelineEntry | undefined>()
  const dialogBusy = ref(false)

  const statusEntry = ref<TimelineEntry | null>(null)
  const statusAction = ref<'validate' | 'lock' | 'withdraw'>('validate')
  const statusReason = ref('')
  const statusBusy = ref(false)

  const reasonRequired = computed(() => statusAction.value === 'withdraw')

  async function load (append = false) {
    loading.value = true
    try {
      const res = await listTimeline(props.caseId, {
        entryTypes: types.value.length > 0 ? types.value : undefined,
        includeWithdrawn: includeWithdrawn.value || undefined,
        pageSize: PAGE_SIZE,
        pageToken: append ? nextToken.value : undefined,
      })
      entries.value = append ? [...entries.value, ...(res.entries ?? [])] : (res.entries ?? [])
      total.value = res.totalSize ?? 0
      nextToken.value = res.nextPageToken ?? ''
      emit('drafts', res.draftCount ?? 0)
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  function openDialog (mode: 'create' | 'edit' | 'correct', entry?: TimelineEntry) {
    dialogMode.value = mode
    dialogEntry.value = entry
    dialogOpen.value = true
  }

  async function saveCreate (sub: EntrySubmission, correctsEntryId?: string) {
    await createTimelineEntry(props.caseId, { ...sub.content, correctsEntryId, documentIds: sub.documents.map(d => d.id) })
  }

  // An edit replaces the content, then applies the document differences.
  async function saveEdit (entry: TimelineEntry, sub: EntrySubmission) {
    await updateTimelineEntry(entry.id, { ...sub.content, reason: sub.reason })
    const before = new Set((entry.documents ?? []).map(d => d.documentId))
    const after = new Set(sub.documents.map(d => d.id))
    for (const id of after) {
      if (!before.has(id)) await linkTimelineDocument(entry.id, id)
    }
    for (const id of before) {
      if (!after.has(id)) await unlinkTimelineDocument(entry.id, id, sub.reason)
    }
  }

  async function submitDialog (sub: EntrySubmission) {
    dialogBusy.value = true
    try {
      const entry = dialogEntry.value
      await (dialogMode.value === 'edit' && entry ? saveEdit(entry, sub) : saveCreate(sub, dialogMode.value === 'correct' ? entry?.id : undefined))
      ui.notify(t(`timeline.messages.${dialogMode.value}`), 'success')
      dialogOpen.value = false
      await load()
      emit('changed')
    } catch (error) {
      report(error)
    } finally {
      dialogBusy.value = false
    }
  }

  function askStatus (entry: TimelineEntry, action: 'validate' | 'lock' | 'withdraw') {
    statusEntry.value = entry
    statusAction.value = action
    statusReason.value = ''
  }

  async function doStatus () {
    const entry = statusEntry.value
    if (!entry || (reasonRequired.value && !statusReason.value.trim())) return
    statusBusy.value = true
    try {
      await changeTimelineEntryStatus(entry.id, statusAction.value, statusReason.value.trim() || undefined)
      ui.notify(t(`timeline.messages.${statusAction.value}`), 'success')
      statusEntry.value = null
      await load()
      emit('changed')
    } catch (error) {
      report(error)
    } finally {
      statusBusy.value = false
    }
  }

  watch([() => props.caseId, () => props.reloadKey, types, includeWithdrawn], () => load(), { immediate: true })
</script>

<template>
  <div>
    <div class="d-flex flex-wrap align-center ga-2 mb-3">
      <v-chip-group
        v-model="types"
        class="flex-grow-1"
        column
        filter
        multiple
      >
        <v-chip
          v-for="type in FILTER_ENTRY_TYPES"
          :key="type"
          :prepend-icon="entryTypeStyle(type).icon"
          size="small"
          :value="type"
          variant="outlined"
        >
          {{ enumLabel('TimelineEntryType', type) }}
        </v-chip>
      </v-chip-group>

      <v-switch
        v-model="includeWithdrawn"
        color="primary"
        density="compact"
        hide-details
        :label="t('timeline.showWithdrawn')"
      />

      <v-btn
        v-if="canEdit"
        color="primary"
        prepend-icon="mdi-plus"
        size="small"
        variant="flat"
        @click="openDialog('create')"
      >
        {{ t('timeline.actions.add') }}
      </v-btn>
    </div>

    <v-progress-linear v-if="loading" class="mb-2" color="primary" indeterminate />

    <p v-if="!loading && entries.length === 0" class="text-medium-emphasis">{{ t('timeline.empty') }}</p>

    <v-timeline
      v-else
      align="start"
      density="compact"
      side="end"
      truncate-line="both"
    >
      <v-timeline-item
        v-for="entry in entries"
        :key="entry.id"
        :dot-color="entryTypeStyle(entry.entryType).color"
        :icon="entryTypeStyle(entry.entryType).icon"
        size="small"
        width="100%"
      >
        <TimelineEntryCard
          :can-edit="canEdit"
          :entry="entry"
          @correct="openDialog('correct', $event)"
          @edit="openDialog('edit', $event)"
          @status="askStatus"
        />
      </v-timeline-item>
    </v-timeline>

    <div v-if="nextToken" class="d-flex justify-center mt-2">
      <v-btn :loading="loading" variant="text" @click="load(true)">
        {{ t('actions.common.loadMore') }} ({{ entries.length }} / {{ total }})
      </v-btn>
    </div>

    <TimelineEntryDialog
      v-model="dialogOpen"
      :busy="dialogBusy"
      :entry="dialogEntry"
      :mode="dialogMode"
      @submit="submitDialog"
    />

    <v-dialog max-width="480" :model-value="!!statusEntry" @update:model-value="statusEntry = null">
      <v-card>
        <v-card-title>{{ t(`timeline.actions.${statusAction}`) }}</v-card-title>

        <v-card-text>
          <p class="mb-3">{{ t(`timeline.confirm.${statusAction}`) }}</p>

          <v-text-field
            v-model="statusReason"
            :label="t('finalize.reason')"
            :rules="reasonRequired ? [required(t)] : []"
          />
        </v-card-text>

        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="statusEntry = null">{{ t('actions.common.cancel') }}</v-btn>

          <v-btn :color="statusAction === 'withdraw' ? 'error' : 'primary'" :loading="statusBusy" variant="flat" @click="doStatus">
            {{ t('actions.common.confirm') }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>
