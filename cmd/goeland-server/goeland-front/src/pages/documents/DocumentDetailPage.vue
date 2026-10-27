<script setup lang="ts">
  import type { AuditEvent, GoDocument } from '@/api/types'
  import type { DocumentFormModel } from '@/components/document/documentForm'
  import type { RoleSection } from '@/utils/roleHeadings'
  import { computed, nextTick, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import {
    deleteDocument,
    downloadDocumentBlob,
    finalizeDocument,
    getDocument,
    linkDocument,
    updateDocumentMetadata,
  } from '@/api/documentClient'
  import AccessPanel from '@/components/access/AccessPanel.vue'
  import AuditTimeline from '@/components/core/AuditTimeline.vue'
  import LinkSubjectDialog from '@/components/core/LinkSubjectDialog.vue'
  import RecordMetadataPanel from '@/components/core/RecordMetadataPanel.vue'
  import RelationshipTable from '@/components/core/RelationshipTable.vue'
  import SubjectIdentityCard from '@/components/core/SubjectIdentityCard.vue'
  import SubjectRolesPanel from '@/components/core/SubjectRolesPanel.vue'
  import UserLabel from '@/components/core/UserLabel.vue'
  import DocumentFinalizeDialog from '@/components/document/DocumentFinalizeDialog.vue'
  import DocumentIntegrityPanel from '@/components/document/DocumentIntegrityPanel.vue'
  import DocumentMetadataForm from '@/components/document/DocumentMetadataForm.vue'
  import DocumentStatusChip from '@/components/document/DocumentStatusChip.vue'
  import DocumentVersionsPanel from '@/components/document/DocumentVersionsPanel.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useMyAccess } from '@/composables/useMyAccess'
  import { usePagedRelationships } from '@/composables/usePagedRelationships'
  import { useSubjectLinks } from '@/composables/useSubjectLinks'
  import { useUiStore } from '@/stores/ui'
  import { documentIcon, downloadable } from '@/utils/documents'
  import { formatBytes, formatDate, formatDateTime } from '@/utils/formatters'
  import { kindIcon } from '@/utils/subjects'

  // The document page (GLD-075, the GLD-063 model): a header with the title,
  // the state, the key facts and the download, jump links with counts, then
  // the cases, actors and objects the document belongs to, grouped by role and
  // counted, beside its versions; the technical side sits in its own tab.
  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const id = computed(() => String(route.params.id))
  // The caller's level (GLD-048) hides what the server would refuse.
  const { access, reload: reloadAccess, canManage, hasFullControl } = useMyAccess(id)
  const doc = ref<GoDocument | null>(null)
  const {
    relationships, hasMore: relHasMore, loading: relLoading, total: relTotal, capped: relCapped,
    sort: relSort, reload: reloadRelationships, loadMore: loadMoreRelationships,
  } = usePagedRelationships(id)
  const audit = ref<AuditEvent[]>([])
  const loading = ref(true)
  const tab = ref<'document' | 'governance'>('document')
  const sections = ref<RoleSection[]>([])
  const rolesKey = ref(0)

  const editing = ref(false)
  const editModel = ref<DocumentFormModel>({ title: '', description: '', officialDate: '', language: '', isRecord: false })
  const editReason = ref('')
  const saving = ref(false)

  const finalizeOpen = ref(false)
  const finalizeBusy = ref(false)
  const downloading = ref(false)
  const { linkOpen, linkBusy, doLink, doUnlink } = useSubjectLinks(id, reload, linkDocument)
  const deleteOpen = ref(false)
  const deleteReason = ref('')
  const deleteBusy = ref(false)

  const isLocked = computed(() => !!doc.value?.recordMetadata?.isLocked)
  const isDeleted = computed(() => !!doc.value?.recordMetadata?.deletedAt)
  const isFinal = computed(() => !!doc.value?.currentVersion?.isFinal)
  const isRecord = computed(() => !!doc.value?.currentVersion?.isRecord)
  const isConfidential = computed(() => (doc.value?.recordMetadata?.confidentialityLevel ?? 0) > 0)
  const editable = computed(() => !isLocked.value && !isDeleted.value && canManage.value)
  const canFinalize = computed(() => !isFinal.value && editable.value)
  const canDelete = computed(() => !isDeleted.value && !isLocked.value && hasFullControl.value)
  const content = computed(() => doc.value?.currentVersion?.content)
  const canDownload = computed(() => downloadable(content.value?.storageRef))
  const icon = computed(() => documentIcon(content.value?.mimeType))
  const fileFacts = computed(() => [content.value?.mimeType, content.value?.fileSizeBytes ? formatBytes(content.value.fileSizeBytes) : '']
    .filter(Boolean)
    .join(' · '))

  async function reload () {
    loading.value = true
    try {
      const res = await getDocument(id.value, { includeAudit: true })
      doc.value = res.document ?? null
      audit.value = res.recentAudit ?? []
      rolesKey.value++
      await reloadRelationships()
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  /** Shows the document tab and scrolls to a section of it. */
  async function goTo (anchor: string) {
    tab.value = 'document'
    await nextTick()
    document.querySelector(`#${anchor}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  async function download () {
    if (!doc.value) return
    downloading.value = true
    try {
      await downloadDocumentBlob(id.value, doc.value.title || 'document', doc.value.currentVersion?.id)
    } catch (error) {
      report(error)
    } finally {
      downloading.value = false
    }
  }

  function startEdit () {
    if (!doc.value) return
    editModel.value = {
      title: doc.value.title ?? '',
      description: doc.value.description ?? '',
      officialDate: doc.value.officialDate ?? '',
      language: doc.value.language ?? '',
      isRecord: isRecord.value,
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

  // Reload on id change too: following a link to another subject reuses this page.
  watch(id, () => {
    tab.value = 'document'
    editing.value = false
    void reload()
  }, { immediate: true })
</script>

<template>
  <v-container class="detail-page" fluid>
    <div class="d-flex align-center ga-1 mb-2 text-body-2 text-medium-emphasis">
      <v-btn
        :aria-label="t('nav.documents')"
        icon="mdi-arrow-left"
        size="small"
        variant="text"
        @click="router.push('/documents')"
      />

      <router-link class="text-medium-emphasis" to="/documents">{{ t('nav.documents') }}</router-link>
      <span aria-hidden="true">›</span>
      <span class="text-truncate">{{ doc?.title }}</span>
    </div>

    <v-progress-linear v-if="loading && !doc" color="primary" indeterminate />

    <template v-if="doc">
      <v-card aria-labelledby="document-title" class="mb-4 pa-5" tag="section">
        <div class="d-flex flex-wrap align-start ga-4">
          <v-avatar color="primary" rounded="lg" size="52" variant="tonal"><v-icon :icon="icon" size="28" /></v-avatar>

          <div class="detail-page__title">
            <h1 id="document-title" class="text-h5 font-weight-medium">{{ doc.title }}</h1>

            <div class="d-flex flex-wrap align-center ga-2 mt-1">
              <v-chip label size="small">{{ doc.documentType?.label ?? doc.documentType?.code }}</v-chip>
              <DocumentStatusChip :status="doc.status" />

              <v-chip
                v-if="isFinal"
                color="success"
                label
                prepend-icon="mdi-check-decagram"
                size="small"
              >{{ t('states.final') }}</v-chip>

              <v-chip v-if="isRecord" color="primary" label size="small">{{ t('states.record') }}</v-chip>

              <v-chip
                v-if="isConfidential"
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

            <p v-if="doc.description" class="text-body-2 text-medium-emphasis mt-3 document-page__description">{{ doc.description }}</p>
          </div>

          <div class="d-flex flex-wrap ga-2">
            <v-btn
              v-if="canDownload"
              color="primary"
              :loading="downloading"
              prepend-icon="mdi-download"
              variant="flat"
              @click="download"
            >{{ t('actions.document.download') }}</v-btn>

            <v-btn
              v-else-if="doc.externalUrl"
              append-icon="mdi-open-in-new"
              color="primary"
              :href="doc.externalUrl"
              rel="noopener noreferrer"
              target="_blank"
              variant="flat"
            >{{ t('documentPage.openExternal', { system: doc.externalSystem || t('documentPage.externalSystem') }) }}</v-btn>

            <v-btn v-if="editable && !editing" prepend-icon="mdi-pencil" variant="outlined" @click="startEdit">{{ t('actions.document.updateMetadata') }}</v-btn>
            <v-btn v-if="editable" prepend-icon="mdi-link-plus" variant="outlined" @click="linkOpen = true">{{ t('detailPage.link') }}</v-btn>

            <v-menu v-if="canFinalize || canDelete" location="bottom end">
              <template #activator="{ props: menu }">
                <v-btn v-bind="menu" :aria-label="t('detailPage.more')" icon="mdi-dots-vertical" variant="text" />
              </template>

              <v-list density="compact">
                <v-list-item v-if="canFinalize" prepend-icon="mdi-check-decagram" :title="t('actions.document.finalize')" @click="finalizeOpen = true" />

                <v-list-item
                  v-if="canDelete"
                  base-color="error"
                  prepend-icon="mdi-delete"
                  :title="t('actions.document.delete')"
                  @click="deleteOpen = true"
                />
              </v-list>
            </v-menu>
          </div>
        </div>

        <div class="detail-page__facts">
          <div><div class="detail-page__label">{{ t('fields.document.official_date') }}</div>{{ formatDate(doc.officialDate) }}</div>

          <div>
            <div class="detail-page__label">{{ t('fields.document.version') }}</div>
            {{ doc.currentVersion?.versionNo ?? '—' }}
            <span v-if="doc.currentVersion?.createdAt" class="text-medium-emphasis">({{ formatDate(doc.currentVersion.createdAt) }})</span>
          </div>

          <div><div class="detail-page__label">{{ t('fields.document.file') }}</div>{{ fileFacts || '—' }}</div>
          <div v-if="doc.language"><div class="detail-page__label">{{ t('fields.document.language') }}</div>{{ doc.language }}</div>

          <div v-if="doc.externalSystem">
            <div class="detail-page__label">{{ t('fields.document.external_system') }}</div>
            <a v-if="doc.externalUrl" :href="doc.externalUrl" rel="noopener noreferrer" target="_blank">{{ doc.externalSystem }}<v-icon class="ml-1" icon="mdi-open-in-new" size="x-small" /><span class="sr-only">({{ t('detailPage.newTab') }})</span></a>
            <template v-else>{{ doc.externalSystem }}</template>
          </div>

          <div>
            <div class="detail-page__label">{{ t('fields.common.created_at') }}</div>
            {{ formatDateTime(doc.createdAt) }}
            <span v-if="doc.createdBy" class="text-medium-emphasis">· <UserLabel :id="doc.createdBy" /></span>
          </div>
        </div>

        <nav v-if="sections.length > 0" aria-labelledby="document-jump" class="mt-4">
          <div id="document-jump" class="detail-page__label mb-1">{{ t('detailPage.jump') }}</div>

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
        v-if="isLocked"
        class="mb-3"
        density="compact"
        type="info"
        variant="tonal"
      >{{ t('messages.document.lockedCannotEdit') }}</v-alert>

      <v-alert
        v-if="isDeleted"
        class="mb-3"
        density="compact"
        type="warning"
        variant="tonal"
      >{{ t('messages.document.deletedReadOnly') }}</v-alert>

      <v-card v-if="editing" class="mb-4">
        <v-card-title class="text-subtitle-1">{{ t('actions.document.updateMetadata') }}</v-card-title>

        <v-card-text>
          <DocumentMetadataForm v-model="editModel" />
          <v-text-field v-model="editReason" class="mt-2" :label="t('finalize.reason')" />

          <div class="d-flex justify-end ga-2">
            <v-btn variant="text" @click="editing = false">{{ t('actions.common.cancel') }}</v-btn>
            <v-btn color="primary" :loading="saving" variant="flat" @click="saveEdit">{{ t('actions.common.save') }}</v-btn>
          </div>
        </v-card-text>
      </v-card>

      <v-tabs v-model="tab" class="mb-4" color="primary">
        <v-tab value="document">{{ t('documentPage.tabs.document') }}</v-tab>
        <v-tab value="governance">{{ t('detailPage.tabs.governance') }}</v-tab>
      </v-tabs>

      <v-window v-model="tab">
        <v-window-item value="document">
          <v-row>
            <v-col cols="12" md="7">
              <SubjectRolesPanel :reload-key="rolesKey" :subject-id="id" subject-kind="SUBJECT_KIND_DOCUMENT" @sections="sections = $event" />
            </v-col>

            <v-col cols="12" md="5">
              <v-card>
                <v-card-title class="text-subtitle-1">{{ t('sections.document.versions') }}</v-card-title>

                <v-card-text>
                  <DocumentVersionsPanel
                    :can-add="editable"
                    :current-version-id="doc.currentVersion?.id"
                    :document-id="id"
                    @changed="reload"
                  />
                </v-card-text>
              </v-card>
            </v-col>
          </v-row>
        </v-window-item>

        <v-window-item value="governance">
          <v-row>
            <v-col cols="12" md="6">
              <SubjectIdentityCard class="mb-4" :subject="doc.subjectRef" />

              <v-card class="mb-4">
                <v-card-title class="text-subtitle-1">{{ t('access.title') }}</v-card-title>
                <v-card-text><AccessPanel :access="access" :subject-id="id" @changed="reloadAccess(); reload()" /></v-card-text>
              </v-card>

              <v-card class="mb-4">
                <v-card-title class="text-subtitle-1">{{ t('sections.document.integrity') }}</v-card-title>
                <v-card-text><DocumentIntegrityPanel :document="doc" @verified="reload" /></v-card-text>
              </v-card>
            </v-col>

            <v-col cols="12" md="6">
              <v-card class="mb-4">
                <v-card-title class="text-subtitle-1">{{ t('sections.document.governance') }}</v-card-title>
                <v-card-text><RecordMetadataPanel :metadata="doc.recordMetadata" /></v-card-text>
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
                    :can-unlink="editable"
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

      <DocumentFinalizeDialog v-model="finalizeOpen" :busy="finalizeBusy" @submit="doFinalize" />

      <LinkSubjectDialog
        v-model="linkOpen"
        :busy="linkBusy"
        :source-id="id"
        source-kind="SUBJECT_KIND_DOCUMENT"
        @submit="doLink"
      />

      <v-dialog v-model="deleteOpen" max-width="480">
        <v-card>
          <v-card-title>{{ t('delete.title') }}</v-card-title>

          <v-card-text>
            <v-alert class="mb-3" density="compact" type="warning" variant="tonal">{{ t('messages.document.deleteConfirm') }}</v-alert>
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

<style scoped>
  /* A long description stays readable without pushing the facts off screen. */
  .document-page__description {
    max-width: 80ch;
    white-space: pre-line;
    display: -webkit-box;
    -webkit-line-clamp: 4;
    line-clamp: 4;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
</style>
