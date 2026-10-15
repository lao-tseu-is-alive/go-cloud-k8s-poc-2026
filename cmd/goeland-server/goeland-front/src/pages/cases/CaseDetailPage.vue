<script setup lang="ts">
  import type { AuditEvent, CaseStatus, GoCase } from '@/api/types'
  import type { RoleSection } from '@/utils/roleHeadings'
  import { computed, nextTick, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { deleteCase, getCase, transitionCase, updateCase } from '@/api/caseClient'
  import AccessPanel from '@/components/access/AccessPanel.vue'
  import CaseExportPanel from '@/components/case/CaseExportPanel.vue'
  import { CASE_TRANSITIONS, transitionNeedsReason } from '@/components/case/caseForm'
  import CaseStatusChip from '@/components/case/CaseStatusChip.vue'
  import CaseCirculationsPanel from '@/components/circulation/CaseCirculationsPanel.vue'
  import AuditTimeline from '@/components/core/AuditTimeline.vue'
  import LinkSubjectDialog from '@/components/core/LinkSubjectDialog.vue'
  import RecordMetadataPanel from '@/components/core/RecordMetadataPanel.vue'
  import RelationshipTable from '@/components/core/RelationshipTable.vue'
  import SubjectIdentityCard from '@/components/core/SubjectIdentityCard.vue'
  import SubjectRolesPanel from '@/components/core/SubjectRolesPanel.vue'
  import CaseTasksPanel from '@/components/task/CaseTasksPanel.vue'
  import CaseTimelinePanel from '@/components/timeline/CaseTimelinePanel.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { useMyAccess } from '@/composables/useMyAccess'
  import { usePagedRelationships } from '@/composables/usePagedRelationships'
  import { useSubjectLinks } from '@/composables/useSubjectLinks'
  import { useUiStore } from '@/stores/ui'
  import { formatDateTime } from '@/utils/formatters'
  import { kindIcon } from '@/utils/subjects'
  import { maxLength, required } from '@/utils/validation'

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const route = useRoute()
  const router = useRouter()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const id = computed(() => String(route.params.id))
  // The caller's level on the case (GLD-048) hides what the server would refuse.
  const { access, reload: reloadAccess, canContribute, canManage, hasFullControl } = useMyAccess(id)
  const current = ref<GoCase | null>(null)
  const {
    relationships, hasMore: relHasMore, loading: relLoading, total: relTotal, capped: relCapped,
    sort: relSort, reload: reloadRelationships, loadMore: loadMoreRelationships,
  } = usePagedRelationships(id)
  const audit = ref<AuditEvent[]>([])
  const loading = ref(true)
  // The case page (GLD-068, the GLD-063 model): the header, then the case's
  // progress (timeline, tasks, circulations), its parties and documents
  // grouped by role, and the technical side in its own tab.
  const tab = ref<'progress' | 'parties' | 'governance'>('progress')
  const sections = ref<RoleSection[]>([])
  const rolesKey = ref(0)

  const editing = ref(false)
  const editTitle = ref('')
  const editDescription = ref('')
  const editReason = ref('')
  const saving = ref(false)

  const transitionTarget = ref<CaseStatus | null>(null)
  const transitionReason = ref('')
  const transitionBusy = ref(false)
  const { linkOpen, linkBusy, doLink, doUnlink } = useSubjectLinks(id, reload)
  const deleteOpen = ref(false)
  const deleteReason = ref('')
  const deleteBusy = ref(false)
  // The timeline reloads when the status changes (SYSTEM entry) and reports its drafts.
  const timelineKey = ref(0)
  const draftCount = ref(0)
  // Tasks report their open count too; completing one adds a timeline entry.
  const openTasks = ref(0)
  // Open circulations block closing too; their answers change tasks and timeline.
  const openCirculations = ref(0)

  // ---- derived state rules -------------------------------------------------
  const isLocked = computed(() => !!current.value?.recordMetadata?.isLocked)
  const isDeleted = computed(() => !!current.value?.recordMetadata?.deletedAt)
  const isClosed = computed(() => current.value?.status === 'CASE_STATUS_CLOSED')
  const mutable = computed(() => !isLocked.value && !isDeleted.value)
  const editable = computed(() => mutable.value && !isClosed.value && canManage.value)
  const contributable = computed(() => mutable.value && !isClosed.value && canContribute.value)
  const linkable = computed(() => mutable.value && canManage.value)
  const transitions = computed(() => (mutable.value && canManage.value && current.value?.status ? CASE_TRANSITIONS[current.value.status] : []))
  const reasonRequired = computed(() => !!transitionTarget.value && transitionNeedsReason(current.value?.status, transitionTarget.value))
  // A case with draft timeline entries or open tasks cannot be closed (server rule).
  const closing = computed(() => transitionTarget.value === 'CASE_STATUS_CLOSED')
  const closingBlocked = computed(() => closing.value && (draftCount.value > 0 || openTasks.value > 0 || openCirculations.value > 0))

  async function onTasksChanged () {
    timelineKey.value++
    await reload()
  }

  async function reload () {
    loading.value = true
    try {
      const res = await getCase(id.value, { includeAudit: true })
      current.value = res.case ?? null
      rolesKey.value++
      await reloadRelationships()
      audit.value = res.recentAudit ?? []
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  function startEdit () {
    if (!current.value) return
    editTitle.value = current.value.title
    editDescription.value = current.value.description ?? ''
    editReason.value = ''
    editing.value = true
  }

  async function saveEdit () {
    saving.value = true
    try {
      await updateCase(id.value, {
        title: editTitle.value.trim(),
        description: editDescription.value.trim(),
        reason: editReason.value.trim() || undefined,
      })
      ui.notify(t('messages.case.updateSuccess'), 'success')
      editing.value = false
      await reload()
    } catch (error) {
      report(error)
    } finally {
      saving.value = false
    }
  }

  function askTransition (target: CaseStatus) {
    transitionTarget.value = target
    transitionReason.value = ''
  }

  async function doTransition () {
    if (!transitionTarget.value || closingBlocked.value) return
    if (reasonRequired.value && !transitionReason.value.trim()) return
    transitionBusy.value = true
    try {
      await transitionCase(id.value, transitionTarget.value, transitionReason.value.trim() || undefined)
      ui.notify(t('messages.case.statusChanged'), 'success')
      transitionTarget.value = null
      timelineKey.value++
      await reload()
    } catch (error) {
      report(error)
    } finally {
      transitionBusy.value = false
    }
  }

  async function doDelete () {
    deleteBusy.value = true
    try {
      await deleteCase(id.value, deleteReason.value.trim())
      ui.notify(t('messages.case.deleted'), 'success')
      deleteOpen.value = false
      await reload()
    } catch (error) {
      report(error)
    } finally {
      deleteBusy.value = false
    }
  }

  /** Shows the parties tab and scrolls to a section of it. */
  async function goTo (anchor: string) {
    tab.value = 'parties'
    await nextTick()
    document.querySelector(`#${anchor}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  // Reload on id change too: following a link to another subject reuses this page.
  watch(id, () => {
    tab.value = 'progress'
    void reload()
  }, { immediate: true })
</script>

<template>
  <v-container class="detail-page" fluid>
    <div class="d-flex align-center ga-1 mb-2 text-body-2 text-medium-emphasis">
      <v-btn
        :aria-label="t('nav.cases')"
        icon="mdi-arrow-left"
        size="small"
        variant="text"
        @click="router.push('/cases')"
      />

      <router-link class="text-medium-emphasis" to="/cases">{{ t('nav.cases') }}</router-link>
      <span aria-hidden="true">›</span>
      <span class="text-truncate">{{ current?.title ?? t('pages.cases.detail.title') }}</span>
    </div>

    <v-progress-linear v-if="loading && !current" color="primary" indeterminate />

    <template v-if="current">
      <v-card aria-labelledby="case-title" class="mb-4 pa-5" tag="section">
        <div class="d-flex flex-wrap align-start ga-4">
          <v-avatar color="primary" rounded="lg" size="52" variant="tonal"><v-icon icon="mdi-briefcase-outline" size="28" /></v-avatar>

          <div class="detail-page__title">
            <div v-if="current.subjectRef?.businessRef" class="detail-page__mono text-medium-emphasis">
              {{ [current.subjectRef.businessRefNamespace, current.subjectRef.businessRef].filter(Boolean).join(' ') }}
            </div>

            <h1 id="case-title" class="text-h5 font-weight-medium">{{ current.title }}</h1>

            <div class="d-flex flex-wrap align-center ga-2 mt-1">
              <CaseStatusChip :status="current.status" />
              <v-chip label size="small">{{ current.caseType?.label ?? current.caseType?.code }}</v-chip>

              <v-chip
                v-if="(current.recordMetadata?.confidentialityLevel ?? 0) > 0"
                color="orange"
                label
                prepend-icon="mdi-eye-off"
                size="small"
              >{{ t('states.confidential') }}</v-chip>

              <v-chip v-if="isLocked" label prepend-icon="mdi-lock" size="small">{{ t('states.locked') }}</v-chip>

              <v-chip
                v-if="isDeleted"
                color="error"
                label
                prepend-icon="mdi-delete"
                size="small"
              >{{ t('states.deleted') }}</v-chip>
            </div>
          </div>

          <div class="d-flex flex-wrap ga-2">
            <v-btn v-if="editable && !editing" prepend-icon="mdi-pencil" variant="outlined" @click="startEdit">{{ t('actions.case.edit') }}</v-btn>

            <v-btn
              v-if="linkable"
              color="primary"
              prepend-icon="mdi-link-plus"
              variant="flat"
              @click="linkOpen = true"
            >{{ t('detailPage.link') }}</v-btn>

            <v-menu v-if="transitions.length > 0" location="bottom end">
              <template #activator="{ props: menu }">
                <v-btn v-bind="menu" append-icon="mdi-menu-down" prepend-icon="mdi-swap-horizontal" variant="tonal">{{ t('casePage.changeStatus') }}</v-btn>
              </template>

              <v-list density="compact">
                <v-list-item
                  v-for="target in transitions"
                  :key="target"
                  :prepend-icon="target === 'CASE_STATUS_CLOSED' ? 'mdi-archive-check' : 'mdi-swap-horizontal'"
                  :title="t('actions.case.moveTo', { status: enumLabel('CaseStatus', target) })"
                  @click="askTransition(target)"
                />
              </v-list>
            </v-menu>

            <v-menu v-if="!isDeleted && !isLocked && hasFullControl" location="bottom end">
              <template #activator="{ props: menu }">
                <v-btn v-bind="menu" :aria-label="t('detailPage.more')" icon="mdi-dots-vertical" variant="text" />
              </template>

              <v-list density="compact">
                <v-list-item base-color="error" prepend-icon="mdi-delete" :title="t('actions.case.delete')" @click="deleteOpen = true" />
              </v-list>
            </v-menu>
          </div>
        </div>

        <p v-if="current.description" class="text-body-2 mt-3 mb-0 case-description">{{ current.description }}</p>

        <div class="detail-page__facts">
          <div><div class="detail-page__label">{{ t('fields.case.opened_at') }}</div>{{ formatDateTime(current.openedAt) }}</div>

          <template v-if="isClosed">
            <div><div class="detail-page__label">{{ t('fields.case.closed_at') }}</div>{{ formatDateTime(current.closedAt) }}</div>
            <div><div class="detail-page__label">{{ t('fields.case.closure_reason') }}</div>{{ current.closureReason || '—' }}</div>
          </template>

          <div><div class="detail-page__label">{{ t('casePage.activity') }}</div>{{ t('casePage.activitySummary', { tasks: openTasks, circulations: openCirculations, drafts: draftCount }) }}</div>
        </div>

        <nav v-if="sections.length > 0" aria-labelledby="case-jump" class="mt-4">
          <div id="case-jump" class="detail-page__label mb-1">{{ t('detailPage.jump') }}</div>

          <div class="d-flex flex-wrap ga-2">
            <a
              v-for="s in sections"
              :key="s.anchor"
              class="detail-page__jump"
              :href="`#${s.anchor}`"
              @click.prevent="goTo(s.anchor)"
            >
              <v-icon :icon="kindIcon(s.kind)" size="small" />
              <span>{{ s.label }}</span>
              <strong>{{ s.total }}</strong>
              <v-icon icon="mdi-arrow-down" size="x-small" />
            </a>
          </div>
        </nav>
      </v-card>

      <v-alert
        v-if="isClosed && mutable"
        class="mb-3"
        density="compact"
        type="info"
        variant="tonal"
      >{{ t('messages.case.closedReadOnly') }}</v-alert>

      <v-alert
        v-if="isDeleted"
        class="mb-3"
        density="compact"
        type="warning"
        variant="tonal"
      >{{ t('messages.case.deletedReadOnly') }}</v-alert>

      <v-card v-if="editing" class="mb-4">
        <v-card-title class="text-subtitle-1">{{ t('actions.case.edit') }}</v-card-title>

        <v-card-text>
          <v-text-field v-model="editTitle" :label="t('fields.case.title')" :rules="[required(t), maxLength(t, 500)]" />

          <v-textarea
            v-model="editDescription"
            auto-grow
            :label="t('fields.case.description')"
            rows="3"
            :rules="[maxLength(t, 4000)]"
          />

          <v-text-field v-model="editReason" :label="t('finalize.reason')" />

          <div class="d-flex justify-end ga-2">
            <v-btn variant="text" @click="editing = false">{{ t('actions.common.cancel') }}</v-btn>
            <v-btn color="primary" :loading="saving" variant="flat" @click="saveEdit">{{ t('actions.common.save') }}</v-btn>
          </div>
        </v-card-text>
      </v-card>

      <v-tabs v-model="tab" class="mb-4" color="primary">
        <v-tab value="progress">{{ t('casePage.tabs.progress') }}</v-tab>
        <v-tab value="parties">{{ t('casePage.tabs.parties') }}</v-tab>
        <v-tab value="governance">{{ t('detailPage.tabs.governance') }}</v-tab>
      </v-tabs>

      <!-- eager: the panels report drafts, open tasks and circulations, which closing needs -->
      <v-window v-model="tab">
        <v-window-item eager value="progress">
          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.case.timeline') }}</v-card-title>

            <v-card-text>
              <CaseTimelinePanel
                :can-edit="contributable"
                :case-id="id"
                :reload-key="timelineKey"
                @changed="reload"
                @drafts="draftCount = $event"
              />
            </v-card-text>
          </v-card>

          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.case.tasks') }}</v-card-title>

            <v-card-text>
              <CaseTasksPanel
                :can-edit="editable"
                :case-id="id"
                :reload-key="timelineKey"
                @changed="onTasksChanged"
                @open="openTasks = $event"
              />
            </v-card-text>
          </v-card>

          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.case.circulations') }}</v-card-title>

            <v-card-text>
              <CaseCirculationsPanel
                :can-edit="editable"
                :case-id="id"
                :reload-key="timelineKey"
                @changed="onTasksChanged"
                @open="openCirculations = $event"
              />
            </v-card-text>
          </v-card>
        </v-window-item>

        <v-window-item eager value="parties">
          <SubjectRolesPanel :reload-key="rolesKey" :subject-id="id" subject-kind="SUBJECT_KIND_CASE" @sections="sections = $event" />
        </v-window-item>

        <v-window-item value="governance">
          <v-row>
            <v-col cols="12" md="6">
              <SubjectIdentityCard class="mb-4" :subject="current.subjectRef" />

              <v-card class="mb-4">
                <v-card-title class="text-subtitle-1">{{ t('access.title') }}</v-card-title>
                <v-card-text><AccessPanel :access="access" :subject-id="id" @changed="reloadAccess(); reload()" /></v-card-text>
              </v-card>

              <v-card v-if="canManage" class="mb-4">
                <v-card-title class="text-subtitle-1">{{ t('export.title') }}</v-card-title>
                <v-card-text><CaseExportPanel :case-id="id" /></v-card-text>
              </v-card>
            </v-col>

            <v-col cols="12" md="6">
              <v-card class="mb-4">
                <v-card-title class="text-subtitle-1">{{ t('sections.document.governance') }}</v-card-title>
                <v-card-text><RecordMetadataPanel :metadata="current.recordMetadata" /></v-card-text>
              </v-card>

              <v-card class="mb-4">
                <v-card-title class="text-subtitle-1">{{ t('sections.document.audit') }}</v-card-title>
                <v-card-text><AuditTimeline :events="audit" /></v-card-text>
              </v-card>
            </v-col>

            <v-col cols="12">
              <v-card>
                <v-card-title class="text-subtitle-1">{{ t('detailPage.allRelationships') }}</v-card-title>

                <v-card-text>
                  <p class="text-caption text-medium-emphasis mb-2">{{ t('detailPage.allRelationshipsHint') }}</p>

                  <RelationshipTable
                    v-model:sort="relSort"
                    :can-unlink="linkable"
                    :capped="relCapped"
                    :has-more="relHasMore"
                    :loading="relLoading"
                    :relationships="relationships"
                    :total="relTotal"
                    @ended="reload"
                    @load-more="loadMoreRelationships"
                    @unlink="doUnlink"
                  />
                </v-card-text>
              </v-card>
            </v-col>
          </v-row>
        </v-window-item>
      </v-window>

      <LinkSubjectDialog
        v-model="linkOpen"
        :busy="linkBusy"
        :source-id="id"
        source-kind="SUBJECT_KIND_CASE"
        @submit="doLink"
      />

      <v-dialog max-width="480" :model-value="!!transitionTarget" @update:model-value="transitionTarget = null">
        <v-card>
          <v-card-title>{{ t('actions.case.moveTo', { status: enumLabel('CaseStatus', transitionTarget ?? undefined) }) }}</v-card-title>

          <v-card-text>
            <v-alert
              v-if="closing && draftCount > 0"
              class="mb-3"
              density="compact"
              type="warning"
              variant="tonal"
            >
              {{ t('messages.case.draftsBlockClosing', { count: draftCount }, draftCount) }}
            </v-alert>

            <v-alert
              v-if="closing && openCirculations > 0"
              class="mb-3"
              density="compact"
              type="warning"
              variant="tonal"
            >
              {{ t('messages.case.circulationsBlockClosing', { count: openCirculations }, openCirculations) }}
            </v-alert>

            <v-alert
              v-if="closing && openTasks > 0"
              class="mb-3"
              density="compact"
              type="warning"
              variant="tonal"
            >
              {{ t('messages.case.tasksBlockClosing', { count: openTasks }, openTasks) }}
            </v-alert>

            <v-text-field
              v-model="transitionReason"
              :hint="reasonRequired ? t('messages.case.reasonRequired') : ''"
              :label="t('finalize.reason')"
              persistent-hint
              :rules="reasonRequired ? [required(t)] : []"
            />
          </v-card-text>

          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="transitionTarget = null">{{ t('actions.common.cancel') }}</v-btn>

            <v-btn
              color="primary"
              :disabled="closingBlocked"
              :loading="transitionBusy"
              variant="flat"
              @click="doTransition"
            >
              {{ t('actions.common.confirm') }}
            </v-btn>
          </v-card-actions>
        </v-card>
      </v-dialog>

      <v-dialog v-model="deleteOpen" max-width="480">
        <v-card>
          <v-card-title>{{ t('delete.title') }}</v-card-title>

          <v-card-text>
            <p class="mb-3">{{ t('messages.case.deleteConfirm') }}</p>
            <v-text-field v-model="deleteReason" :label="t('finalize.reason')" />
          </v-card-text>

          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="deleteOpen = false">{{ t('actions.common.cancel') }}</v-btn>
            <v-btn color="error" :loading="deleteBusy" variant="flat" @click="doDelete">{{ t('actions.common.confirm') }}</v-btn>
          </v-card-actions>
        </v-card>
      </v-dialog>
    </template>
  </v-container>
</template>

<style scoped>
  .case-description {
    white-space: pre-line;
    max-width: 80ch;
  }
</style>
