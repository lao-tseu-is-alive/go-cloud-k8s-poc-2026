<script setup lang="ts">
  import type { AuditEvent, CaseStatus, GoCase, SubjectRelationship } from '@/api/types'
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { deleteCase, getCase, transitionCase, updateCase } from '@/api/caseClient'
  import { linkSubjects, unlinkSubjects } from '@/api/coreClient'
  import { CASE_TRANSITIONS, transitionNeedsReason } from '@/components/case/caseForm'
  import CaseStatusChip from '@/components/case/CaseStatusChip.vue'
  import AuditTimeline from '@/components/core/AuditTimeline.vue'
  import LinkSubjectDialog from '@/components/core/LinkSubjectDialog.vue'
  import RecordMetadataPanel from '@/components/core/RecordMetadataPanel.vue'
  import RelationshipTable from '@/components/core/RelationshipTable.vue'
  import SubjectIdentityCard from '@/components/core/SubjectIdentityCard.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { useUiStore } from '@/stores/ui'
  import { formatDateTime } from '@/utils/formatters'
  import { maxLength, required } from '@/utils/validation'

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const route = useRoute()
  const router = useRouter()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const id = computed(() => String(route.params.id))
  const current = ref<GoCase | null>(null)
  const relationships = ref<SubjectRelationship[]>([])
  const audit = ref<AuditEvent[]>([])
  const loading = ref(true)

  const editing = ref(false)
  const editTitle = ref('')
  const editDescription = ref('')
  const editReason = ref('')
  const saving = ref(false)

  const transitionTarget = ref<CaseStatus | null>(null)
  const transitionReason = ref('')
  const transitionBusy = ref(false)
  const linkOpen = ref(false)
  const linkBusy = ref(false)
  const deleteOpen = ref(false)
  const deleteReason = ref('')
  const deleteBusy = ref(false)

  // ---- derived state rules -------------------------------------------------
  const isLocked = computed(() => !!current.value?.recordMetadata?.isLocked)
  const isDeleted = computed(() => !!current.value?.recordMetadata?.deletedAt)
  const isClosed = computed(() => current.value?.status === 'CASE_STATUS_CLOSED')
  const mutable = computed(() => !isLocked.value && !isDeleted.value)
  const editable = computed(() => mutable.value && !isClosed.value)
  const transitions = computed(() => (mutable.value && current.value?.status ? CASE_TRANSITIONS[current.value.status] : []))
  const reasonRequired = computed(() => !!transitionTarget.value && transitionNeedsReason(current.value?.status, transitionTarget.value))

  async function reload () {
    loading.value = true
    try {
      const res = await getCase(id.value, { includeRelationships: true, includeAudit: true })
      current.value = res.case ?? null
      relationships.value = res.relationships ?? []
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
    if (!transitionTarget.value) return
    if (reasonRequired.value && !transitionReason.value.trim()) return
    transitionBusy.value = true
    try {
      await transitionCase(id.value, transitionTarget.value, transitionReason.value.trim() || undefined)
      ui.notify(t('messages.case.statusChanged'), 'success')
      transitionTarget.value = null
      await reload()
    } catch (error) {
      report(error)
    } finally {
      transitionBusy.value = false
    }
  }

  async function doLink (payload: { targetSubjectId: string, relationshipTypeCode: string, roleDetail: string }) {
    linkBusy.value = true
    try {
      await linkSubjects(id.value, payload.targetSubjectId, payload.relationshipTypeCode, payload.roleDetail || undefined)
      ui.notify(t('messages.document.linked'), 'success')
      linkOpen.value = false
      await reload()
    } catch (error) {
      report(error)
    } finally {
      linkBusy.value = false
    }
  }

  async function doUnlink (rel: SubjectRelationship) {
    try {
      await unlinkSubjects(rel.id, '')
      await reload()
    } catch (error) {
      report(error)
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

  // Reload on id change too: following a link to another subject reuses this page.
  watch(id, reload, { immediate: true })
</script>

<template>
  <v-container fluid>
    <div class="d-flex align-center ga-2 mb-2">
      <v-btn icon="mdi-arrow-left" variant="text" @click="router.push('/cases')" />
      <h1 class="text-h5 text-truncate">{{ current?.title ?? t('pages.cases.detail.title') }}</h1>
    </div>

    <v-progress-linear v-if="loading" color="primary" indeterminate />

    <template v-if="current">
      <div class="d-flex flex-wrap ga-2 mb-3 align-center">
        <CaseStatusChip :status="current.status" />
        <v-chip v-if="isLocked" color="grey" prepend-icon="mdi-lock" size="small">{{ t('states.locked') }}</v-chip>
        <v-chip v-if="isDeleted" color="error" prepend-icon="mdi-delete" size="small">{{ t('states.deleted') }}</v-chip>
      </div>

      <v-alert
        v-if="isClosed && mutable"
        class="mb-3"
        density="compact"
        type="info"
        variant="tonal"
      >
        {{ t('messages.case.closedReadOnly') }}
      </v-alert>

      <v-alert
        v-if="isDeleted"
        class="mb-3"
        density="compact"
        type="warning"
        variant="tonal"
      >
        {{ t('messages.case.deletedReadOnly') }}
      </v-alert>

      <!-- action bar: metadata, lifecycle transitions, deletion -->
      <div class="d-flex flex-wrap ga-2 mb-4">
        <v-btn
          v-if="editable && !editing"
          color="primary"
          prepend-icon="mdi-pencil"
          variant="tonal"
          @click="startEdit"
        >
          {{ t('actions.case.edit') }}
        </v-btn>

        <v-btn
          v-for="target in transitions"
          :key="target"
          :prepend-icon="target === 'CASE_STATUS_CLOSED' ? 'mdi-archive-check' : 'mdi-swap-horizontal'"
          variant="tonal"
          @click="askTransition(target)"
        >
          {{ t('actions.case.moveTo', { status: enumLabel('CaseStatus', target) }) }}
        </v-btn>

        <v-btn
          v-if="!isDeleted && !isLocked"
          color="error"
          prepend-icon="mdi-delete"
          variant="text"
          @click="deleteOpen = true"
        >
          {{ t('actions.case.delete') }}
        </v-btn>
      </div>

      <v-row>
        <v-col cols="12" md="8">
          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.case.summary') }}</v-card-title>

            <v-card-text>
              <template v-if="editing">
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
              </template>

              <v-table v-else density="compact">
                <tbody>
                  <tr><td class="text-medium-emphasis" style="width:40%">{{ t('fields.case.case_type') }}</td><td>{{ current.caseType?.label ?? current.caseType?.code }}</td></tr>
                  <tr><td class="text-medium-emphasis">{{ t('fields.case.description') }}</td><td>{{ current.description || '—' }}</td></tr>
                  <tr><td class="text-medium-emphasis">{{ t('fields.case.opened_at') }}</td><td>{{ formatDateTime(current.openedAt) }}</td></tr>

                  <template v-if="isClosed">
                    <tr><td class="text-medium-emphasis">{{ t('fields.case.closed_at') }}</td><td>{{ formatDateTime(current.closedAt) }}</td></tr>
                    <tr><td class="text-medium-emphasis">{{ t('fields.case.closure_reason') }}</td><td>{{ current.closureReason || '—' }}</td></tr>
                  </template>
                </tbody>
              </v-table>
            </v-card-text>
          </v-card>

          <v-card class="mb-4">
            <v-card-title class="d-flex align-center text-subtitle-1">
              {{ t('sections.case.relationships') }}
              <v-spacer />

              <v-btn
                v-if="mutable"
                prepend-icon="mdi-link-plus"
                size="small"
                variant="tonal"
                @click="linkOpen = true"
              >
                {{ t('actions.document.link') }}
              </v-btn>
            </v-card-title>

            <v-card-text>
              <p class="text-caption text-medium-emphasis mb-2">{{ t('messages.case.relationshipsHint') }}</p>

              <RelationshipTable
                :can-unlink="mutable"
                :relationships="relationships"
                @ended="reload"
                @unlink="doUnlink"
              />
            </v-card-text>
          </v-card>
        </v-col>

        <v-col cols="12" md="4">
          <SubjectIdentityCard class="mb-4" :subject="current.subjectRef" />

          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.document.governance') }}</v-card-title>
            <v-card-text><RecordMetadataPanel :metadata="current.recordMetadata" /></v-card-text>
          </v-card>

          <v-card>
            <v-card-title class="text-subtitle-1">{{ t('sections.document.audit') }}</v-card-title>
            <v-card-text><AuditTimeline :events="audit" /></v-card-text>
          </v-card>
        </v-col>
      </v-row>

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
            <v-btn color="primary" :loading="transitionBusy" variant="flat" @click="doTransition">{{ t('actions.common.confirm') }}</v-btn>
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
