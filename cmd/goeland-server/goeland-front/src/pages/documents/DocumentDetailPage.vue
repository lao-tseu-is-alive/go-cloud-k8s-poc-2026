<script setup lang="ts">
  import type { AuditEvent, GoDocument, SubjectRelationship } from '@/api/types'
  import type { DocumentFormModel } from '@/components/document/documentForm'
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { unlinkSubjects } from '@/api/coreClient'
  import {
    deleteDocument,
    finalizeDocument,
    getDocument,
    linkDocument,
    updateDocumentMetadata,
  } from '@/api/documentClient'
  import LinkSubjectDialog from '@/components/core/LinkSubjectDialog.vue'
  import RecordMetadataPanel from '@/components/core/RecordMetadataPanel.vue'
  import SubjectIdentityCard from '@/components/core/SubjectIdentityCard.vue'
  import DocumentAuditPanel from '@/components/document/DocumentAuditPanel.vue'
  import DocumentFinalizeDialog from '@/components/document/DocumentFinalizeDialog.vue'
  import DocumentIntegrityPanel from '@/components/document/DocumentIntegrityPanel.vue'
  import DocumentMetadataForm from '@/components/document/DocumentMetadataForm.vue'
  import DocumentRelationshipsPanel from '@/components/document/DocumentRelationshipsPanel.vue'
  import DocumentStatusChip from '@/components/document/DocumentStatusChip.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useUiStore } from '@/stores/ui'
  import { formatBytes } from '@/utils/formatters'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const id = computed(() => String(route.params.id))
  const doc = ref<GoDocument | null>(null)
  const relationships = ref<SubjectRelationship[]>([])
  const audit = ref<AuditEvent[]>([])
  const loading = ref(true)

  const editing = ref(false)
  const editModel = ref<DocumentFormModel>({ title: '', description: '', officialDate: '', language: '', isRecord: false })
  const editReason = ref('')
  const saving = ref(false)

  const finalizeOpen = ref(false)
  const finalizeBusy = ref(false)
  const linkOpen = ref(false)
  const linkBusy = ref(false)
  const deleteOpen = ref(false)
  const deleteReason = ref('')
  const deleteBusy = ref(false)

  // ---- derived state rules -------------------------------------------------
  const isLocked = computed(() => !!doc.value?.recordMetadata?.isLocked)
  const isDeleted = computed(() => !!doc.value?.recordMetadata?.deletedAt)
  const isFinal = computed(() => !!doc.value?.isFinal)
  const editable = computed(() => !isLocked.value && !isDeleted.value)
  const canFinalize = computed(() => !isFinal.value && editable.value)
  const canDelete = computed(() => !isDeleted.value && !isLocked.value)
  const isConfidential = computed(() => (doc.value?.recordMetadata?.confidentialityLevel ?? 0) > 0)

  async function reload () {
    loading.value = true
    try {
      const res = await getDocument(id.value, { includeRelationships: true, includeAudit: true })
      doc.value = res.document ?? null
      relationships.value = res.relationships ?? []
      audit.value = res.recentAudit ?? []
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  function startEdit () {
    if (!doc.value) return
    editModel.value = {
      title: doc.value.title ?? '',
      description: doc.value.description ?? '',
      officialDate: doc.value.officialDate ?? '',
      language: doc.value.language ?? '',
      isRecord: !!doc.value.isRecord,
    }
    editReason.value = ''
    editing.value = true
  }

  async function saveEdit () {
    saving.value = true
    try {
      await updateDocumentMetadata(id.value, {
        title: editModel.value.title.trim(),
        description: editModel.value.description.trim(),
        officialDate: editModel.value.officialDate || undefined,
        language: editModel.value.language.trim() || undefined,
        reason: editReason.value.trim() || undefined,
      })
      ui.notify(t('messages.document.updateSuccess'), 'success')
      editing.value = false
      await reload()
    } catch (error) {
      report(error)
    } finally {
      saving.value = false
    }
  }

  async function doFinalize (payload: { reason: string, alsoLock: boolean }) {
    finalizeBusy.value = true
    try {
      await finalizeDocument(id.value, payload.reason, payload.alsoLock)
      ui.notify(t('messages.document.finalizeSuccess'), 'success')
      finalizeOpen.value = false
      await reload()
    } catch (error) {
      report(error)
    } finally {
      finalizeBusy.value = false
    }
  }

  async function doLink (payload: { targetSubjectId: string, relationshipTypeCode: string, roleDetail: string }) {
    linkBusy.value = true
    try {
      await linkDocument(id.value, payload.targetSubjectId, payload.relationshipTypeCode, payload.roleDetail || undefined)
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
      await deleteDocument(id.value, deleteReason.value.trim())
      ui.notify(t('messages.document.deleted'), 'success')
      deleteOpen.value = false
      await reload()
    } catch (error) {
      report(error)
    } finally {
      deleteBusy.value = false
    }
  }

  onMounted(reload)
</script>

<template>
  <v-container fluid>
    <div class="d-flex align-center ga-2 mb-2">
      <v-btn icon="mdi-arrow-left" variant="text" @click="router.push('/documents')" />
      <h1 class="text-h5 text-truncate">{{ doc?.title ?? t('pages.documents.detail.title') }}</h1>
    </div>

    <v-progress-linear v-if="loading" color="primary" indeterminate />

    <template v-if="doc">
      <!-- state banners + chips -->
      <div class="d-flex flex-wrap ga-2 mb-3 align-center">
        <DocumentStatusChip :status="doc.status" />
        <v-chip v-if="isFinal" color="success" prepend-icon="mdi-check-decagram" size="small">{{ t('states.final') }}</v-chip>
        <v-chip v-if="doc.isRecord" color="primary" size="small">{{ t('states.record') }}</v-chip>
        <v-chip v-if="isConfidential" color="orange" prepend-icon="mdi-eye-off" size="small">{{ t('states.confidential') }}</v-chip>
        <v-chip v-if="isLocked" color="grey" prepend-icon="mdi-lock" size="small">{{ t('states.locked') }}</v-chip>
        <v-chip v-if="isDeleted" color="error" prepend-icon="mdi-delete" size="small">{{ t('states.deleted') }}</v-chip>
      </div>

      <v-alert
        v-if="isLocked"
        class="mb-3"
        density="compact"
        type="info"
        variant="tonal"
      >
        {{ t('messages.document.lockedCannotEdit') }}
      </v-alert>

      <v-alert
        v-if="isDeleted"
        class="mb-3"
        density="compact"
        type="warning"
        variant="tonal"
      >
        {{ t('messages.document.deletedReadOnly') }}
      </v-alert>

      <!-- action bar -->
      <div class="d-flex flex-wrap ga-2 mb-4">
        <v-btn
          v-if="editable && !editing"
          color="primary"
          prepend-icon="mdi-pencil"
          variant="tonal"
          @click="startEdit"
        >
          {{ t('actions.document.updateMetadata') }}
        </v-btn>

        <v-btn
          v-if="canFinalize"
          color="warning"
          prepend-icon="mdi-check-decagram"
          variant="tonal"
          @click="finalizeOpen = true"
        >
          {{ t('actions.document.finalize') }}
        </v-btn>

        <v-btn
          v-if="canDelete"
          color="error"
          prepend-icon="mdi-delete"
          variant="text"
          @click="deleteOpen = true"
        >
          {{ t('actions.document.delete') }}
        </v-btn>
      </div>

      <v-row>
        <!-- main column -->
        <v-col cols="12" md="8">
          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.document.summary') }}</v-card-title>

            <v-card-text>
              <template v-if="editing">
                <DocumentMetadataForm v-model="editModel" />
                <v-text-field v-model="editReason" class="mt-2" :label="t('finalize.reason')" />

                <div class="d-flex justify-end ga-2">
                  <v-btn variant="text" @click="editing = false">{{ t('actions.common.cancel') }}</v-btn>
                  <v-btn color="primary" :loading="saving" variant="flat" @click="saveEdit">{{ t('actions.common.save') }}</v-btn>
                </div>
              </template>

              <v-table v-else density="compact">
                <tbody>
                  <tr><td class="text-medium-emphasis" style="width:40%">{{ t('fields.document.description') }}</td><td>{{ doc.description || '—' }}</td></tr>
                  <tr><td class="text-medium-emphasis">{{ t('fields.document.document_type') }}</td><td>{{ doc.documentType?.label ?? doc.documentType?.code }}</td></tr>
                  <tr><td class="text-medium-emphasis">{{ t('fields.document.official_date') }}</td><td>{{ doc.officialDate || '—' }}</td></tr>
                  <tr><td class="text-medium-emphasis">{{ t('fields.document.language') }}</td><td>{{ doc.language || '—' }}</td></tr>
                  <tr><td class="text-medium-emphasis">{{ t('fields.document.version') }}</td><td>{{ doc.version ?? '—' }}</td></tr>
                </tbody>
              </v-table>
            </v-card-text>
          </v-card>

          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.document.storage') }}</v-card-title>

            <v-card-text>
              <v-table density="compact">
                <tbody>
                  <tr><td class="text-medium-emphasis" style="width:40%">{{ t('fields.document.mime_type') }}</td><td>{{ doc.mimeType || '—' }}</td></tr>
                  <tr><td class="text-medium-emphasis">{{ t('fields.document.file_size_bytes') }}</td><td>{{ formatBytes(doc.fileSizeBytes) }}</td></tr>
                  <tr><td class="text-medium-emphasis">{{ t('fields.document.external_system') }}</td><td>{{ doc.externalSystem || '—' }}</td></tr>

                  <tr><td class="text-medium-emphasis">{{ t('fields.document.external_url') }}</td>

                    <td>
                      <a v-if="doc.externalUrl" :href="doc.externalUrl" rel="noopener" target="_blank">{{ doc.externalUrl }}</a>
                      <span v-else>—</span>
                    </td></tr>
                </tbody>
              </v-table>
            </v-card-text>
          </v-card>

          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.document.integrity') }}</v-card-title>

            <v-card-text>
              <DocumentIntegrityPanel :document="doc" @verified="reload" />
            </v-card-text>
          </v-card>

          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.document.relationships') }}</v-card-title>

            <v-card-text>
              <DocumentRelationshipsPanel
                :can-manage="editable"
                :relationships="relationships"
                @add-link="linkOpen = true"
                @unlink="doUnlink"
              />
            </v-card-text>
          </v-card>
        </v-col>

        <!-- side column: governance + audit -->
        <v-col cols="12" md="4">
          <SubjectIdentityCard class="mb-4" :subject="doc.subjectRef" />

          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.document.governance') }}</v-card-title>
            <v-card-text><RecordMetadataPanel :metadata="doc.recordMetadata" /></v-card-text>
          </v-card>

          <v-card>
            <v-card-title class="text-subtitle-1">{{ t('sections.document.audit') }}</v-card-title>
            <v-card-text><DocumentAuditPanel :events="audit" /></v-card-text>
          </v-card>
        </v-col>
      </v-row>

      <!-- dialogs -->
      <DocumentFinalizeDialog v-model="finalizeOpen" :busy="finalizeBusy" @submit="doFinalize" />
      <LinkSubjectDialog v-model="linkOpen" :busy="linkBusy" source-kind="SUBJECT_KIND_DOCUMENT" @submit="doLink" />

      <v-dialog v-model="deleteOpen" max-width="480">
        <v-card>
          <v-card-title>{{ t('delete.title') }}</v-card-title>

          <v-card-text>
            <v-alert class="mb-3" density="compact" type="warning" variant="tonal">
              {{ t('messages.document.deleteConfirm') }}
            </v-alert>

            <v-text-field v-model="deleteReason" :label="t('delete.reason')" />
          </v-card-text>

          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="deleteOpen = false">{{ t('actions.common.cancel') }}</v-btn>
            <v-btn color="error" :loading="deleteBusy" variant="flat" @click="doDelete">{{ t('actions.document.delete') }}</v-btn>
          </v-card-actions>
        </v-card>
      </v-dialog>
    </template>
  </v-container>
</template>
