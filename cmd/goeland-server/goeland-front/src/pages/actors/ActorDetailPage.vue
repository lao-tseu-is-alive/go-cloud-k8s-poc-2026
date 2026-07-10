<script setup lang="ts">
  import type { AuditEvent, GoActor, SubjectRelationship } from '@/api/types'
  import { computed, onMounted, ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { deleteActor, getActor, updateActor } from '@/api/actorClient'
  import ActorContactsPanel from '@/components/actor/ActorContactsPanel.vue'
  import { actorToForm, buildUpdateRequest, emptyActorForm } from '@/components/actor/actorForm'
  import ActorMainForm from '@/components/actor/ActorMainForm.vue'
  import AuditTimeline from '@/components/core/AuditTimeline.vue'
  import RecordMetadataPanel from '@/components/core/RecordMetadataPanel.vue'
  import RelationshipTable from '@/components/core/RelationshipTable.vue'
  import SubjectIdentityCard from '@/components/core/SubjectIdentityCard.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { useUiStore } from '@/stores/ui'

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const route = useRoute()
  const router = useRouter()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const id = computed(() => String(route.params.id))
  const actor = ref<GoActor | null>(null)
  const relationships = ref<SubjectRelationship[]>([])
  const audit = ref<AuditEvent[]>([])
  const loading = ref(true)

  const editing = ref(false)
  const editModel = ref(emptyActorForm())
  const editReason = ref('')
  const saving = ref(false)
  const togglingActive = ref(false)

  const deleteOpen = ref(false)
  const deleteReason = ref('')
  const deleteBusy = ref(false)

  const isLocked = computed(() => !!actor.value?.recordMetadata?.isLocked)
  const isDeleted = computed(() => !!actor.value?.recordMetadata?.deletedAt)
  const isActive = computed(() => !!actor.value?.isActive)
  const editable = computed(() => !isLocked.value && !isDeleted.value)
  const isOrganization = computed(() => actor.value?.actorKind === 'ACTOR_KIND_ORGANIZATION')
  const isConfidential = computed(() => (actor.value?.recordMetadata?.confidentialityLevel ?? 0) > 0)

  async function reload () {
    loading.value = true
    try {
      const res = await getActor(id.value, { includeRelationships: true, includeAudit: true })
      actor.value = res.actor ?? null
      relationships.value = res.relationships ?? []
      audit.value = res.recentAudit ?? []
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  function startEdit () {
    if (!actor.value) return
    editModel.value = actorToForm(actor.value)
    editReason.value = ''
    editing.value = true
  }

  async function saveEdit () {
    saving.value = true
    try {
      await updateActor(id.value, buildUpdateRequest(editModel.value, editReason.value))
      ui.notify(t('messages.actor.updateSuccess'), 'success')
      editing.value = false
      await reload()
    } catch (error) {
      report(error)
    } finally {
      saving.value = false
    }
  }

  async function toggleActive () {
    togglingActive.value = true
    try {
      await updateActor(id.value, { isActive: !isActive.value })
      ui.notify(t('messages.actor.updateSuccess'), 'success')
      await reload()
    } catch (error) {
      report(error)
    } finally {
      togglingActive.value = false
    }
  }

  async function doDelete () {
    deleteBusy.value = true
    try {
      await deleteActor(id.value, deleteReason.value.trim())
      ui.notify(t('messages.actor.deleted'), 'success')
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
      <v-btn icon="mdi-arrow-left" variant="text" @click="router.push('/actors')" />
      <h1 class="text-h5 text-truncate">{{ actor?.displayName ?? t('pages.actors.detail.title') }}</h1>
    </div>

    <v-progress-linear v-if="loading" color="primary" indeterminate />

    <template v-if="actor">
      <div class="d-flex flex-wrap ga-2 mb-3 align-center">
        <v-chip
          :color="isOrganization ? 'indigo' : 'teal'"
          label
          size="small"
        >
          {{ enumLabel('ActorKind', actor.actorKind) }}
        </v-chip>

        <v-chip :color="isActive ? 'success' : 'grey'" size="small">
          {{ isActive ? t('states.active') : t('states.inactive') }}
        </v-chip>

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
        {{ t('messages.actor.lockedCannotEdit') }}
      </v-alert>

      <v-alert
        v-if="isDeleted"
        class="mb-3"
        density="compact"
        type="warning"
        variant="tonal"
      >
        {{ t('messages.actor.deletedReadOnly') }}
      </v-alert>

      <div class="d-flex flex-wrap ga-2 mb-4">
        <v-btn
          v-if="editable && !editing"
          color="primary"
          prepend-icon="mdi-pencil"
          variant="tonal"
          @click="startEdit"
        >
          {{ t('actions.actor.edit') }}
        </v-btn>

        <v-btn
          v-if="editable && !editing"
          :loading="togglingActive"
          prepend-icon="mdi-account-switch"
          variant="text"
          @click="toggleActive"
        >
          {{ isActive ? t('actions.actor.deactivate') : t('actions.actor.activate') }}
        </v-btn>

        <v-btn
          v-if="!isDeleted && !isLocked"
          color="error"
          prepend-icon="mdi-delete"
          variant="text"
          @click="deleteOpen = true"
        >
          {{ t('actions.actor.delete') }}
        </v-btn>
      </div>

      <v-row>
        <v-col cols="12" md="8">
          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.actor.identity') }}</v-card-title>

            <v-card-text>
              <template v-if="editing">
                <ActorMainForm v-model="editModel" lock-kind />
                <v-text-field v-model="editReason" class="mt-2" :label="t('delete.reason')" />

                <div class="d-flex justify-end ga-2">
                  <v-btn variant="text" @click="editing = false">{{ t('actions.common.cancel') }}</v-btn>
                  <v-btn color="primary" :loading="saving" variant="flat" @click="saveEdit">{{ t('actions.common.save') }}</v-btn>
                </div>
              </template>

              <v-table v-else density="compact">
                <tbody>
                  <tr><td class="text-medium-emphasis" style="width:40%">{{ t('fields.actor.actor_kind') }}</td><td>{{ enumLabel('ActorKind', actor.actorKind) }}</td></tr>

                  <template v-if="isOrganization">
                    <tr><td class="text-medium-emphasis">{{ t('fields.actor.legal_name') }}</td><td>{{ actor.organization?.legalName || '—' }}</td></tr>
                    <tr><td class="text-medium-emphasis">{{ t('fields.actor.category') }}</td><td>{{ actor.organization?.categoryCode || '—' }}</td></tr>
                    <tr><td class="text-medium-emphasis">{{ t('fields.actor.org_complement') }}</td><td>{{ actor.organization?.complement || '—' }}</td></tr>
                  </template>

                  <template v-else>
                    <tr><td class="text-medium-emphasis">{{ t('fields.actor.is_ch_register') }}</td><td>{{ actor.person?.isChRegister ? '✓' : '—' }}</td></tr>
                    <tr v-if="actor.person?.isChRegister"><td class="text-medium-emphasis">{{ t('fields.actor.ch_register_ref') }}</td><td>{{ actor.person?.chRegisterRef || '—' }}</td></tr>
                  </template>

                  <tr><td class="text-medium-emphasis">{{ t('fields.actor.publication_code') }}</td><td>{{ actor.publicationCode || '—' }}</td></tr>
                </tbody>
              </v-table>
            </v-card-text>
          </v-card>

          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.actor.contacts') }}</v-card-title>
            <v-card-text><ActorContactsPanel :contacts="actor.contacts" /></v-card-text>
          </v-card>

          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.actor.relationships') }}</v-card-title>

            <v-card-text>
              <p class="text-caption text-medium-emphasis mb-2">{{ t('messages.actor.relationshipsHint') }}</p>
              <RelationshipTable :relationships="relationships" />
            </v-card-text>
          </v-card>
        </v-col>

        <v-col cols="12" md="4">
          <SubjectIdentityCard class="mb-4" :subject="actor.subjectRef" />

          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.actor.governance') }}</v-card-title>
            <v-card-text><RecordMetadataPanel :metadata="actor.recordMetadata" /></v-card-text>
          </v-card>

          <v-card>
            <v-card-title class="text-subtitle-1">{{ t('sections.actor.audit') }}</v-card-title>
            <v-card-text><AuditTimeline :events="audit" /></v-card-text>
          </v-card>
        </v-col>
      </v-row>

      <v-dialog v-model="deleteOpen" max-width="480">
        <v-card>
          <v-card-title>{{ t('actions.actor.delete') }}</v-card-title>

          <v-card-text>
            <v-alert class="mb-3" density="compact" type="warning" variant="tonal">
              {{ t('messages.actor.deleteConfirm') }}
            </v-alert>

            <v-text-field v-model="deleteReason" :label="t('delete.reason')" />
          </v-card-text>

          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="deleteOpen = false">{{ t('actions.common.cancel') }}</v-btn>
            <v-btn color="error" :loading="deleteBusy" variant="flat" @click="doDelete">{{ t('actions.actor.delete') }}</v-btn>
          </v-card-actions>
        </v-card>
      </v-dialog>
    </template>
  </v-container>
</template>
