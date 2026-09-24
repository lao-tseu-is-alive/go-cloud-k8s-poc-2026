<script setup lang="ts">
  import type { AuditEvent, GoThing, SubjectRelationship } from '@/api/types'
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { deleteThing, getThing, updateThing } from '@/api/thingClient'
  import AuditTimeline from '@/components/core/AuditTimeline.vue'
  import LinkSubjectDialog from '@/components/core/LinkSubjectDialog.vue'
  import RecordMetadataPanel from '@/components/core/RecordMetadataPanel.vue'
  import RelationshipTable from '@/components/core/RelationshipTable.vue'
  import SubjectIdentityCard from '@/components/core/SubjectIdentityCard.vue'
  import GeometryPreview from '@/components/thing/GeometryPreview.vue'
  import { buildUpdateThingRequest, emptyThingForm, thingToForm } from '@/components/thing/thingForm'
  import ThingMainForm from '@/components/thing/ThingMainForm.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { useSubjectLinks } from '@/composables/useSubjectLinks'
  import { useUiStore } from '@/stores/ui'
  import { swissMapPointUrl } from '@/utils/geometry'

  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const route = useRoute()
  const router = useRouter()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const id = computed(() => String(route.params.id))
  const thing = ref<GoThing | null>(null)
  const relationships = ref<SubjectRelationship[]>([])
  const audit = ref<AuditEvent[]>([])
  const loading = ref(true)
  const editing = ref(false)
  const editModel = ref(emptyThingForm())
  const editReason = ref('')
  const editForm = ref()
  const saving = ref(false)
  const deleteOpen = ref(false)
  const deleteReason = ref('')
  const deleteBusy = ref(false)
  const { linkOpen, linkBusy, doLink, doUnlink } = useSubjectLinks(id, reload)

  const isDeleted = computed(() => !!thing.value?.recordMetadata?.deletedAt)
  const editable = computed(() => !thing.value?.recordMetadata?.isLocked && !isDeleted.value)
  const mapUrl = computed(() => swissMapPointUrl(thing.value?.anchorE, thing.value?.anchorN))

  async function reload () {
    loading.value = true
    try {
      const res = await getThing(id.value, { includeRelationships: true, includeAudit: true })
      thing.value = res.thing ?? null
      relationships.value = res.relationships ?? []
      audit.value = res.recentAudit ?? []
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }
  // Reload on id change too: following a link to another subject reuses this page.
  watch(id, reload, { immediate: true })

  function startEdit () {
    if (!thing.value) return
    editModel.value = thingToForm(thing.value)
    editReason.value = ''
    editing.value = true
  }

  async function saveEdit () {
    const validation = await editForm.value?.validate()
    if (validation && !validation.valid) return
    saving.value = true
    try {
      await updateThing(id.value, buildUpdateThingRequest(editModel.value, editReason.value))
      ui.notify(t('messages.thing.updateSuccess'), 'success')
      editing.value = false
      await reload()
    } catch (error) {
      report(error)
    } finally {
      saving.value = false
    }
  }

  async function doDelete () {
    deleteBusy.value = true
    try {
      await deleteThing(id.value, deleteReason.value.trim())
      ui.notify(t('messages.thing.deleted'), 'success')
      deleteOpen.value = false
      await reload()
    } catch (error) {
      report(error)
    } finally {
      deleteBusy.value = false
    }
  }
</script>

<template>
  <v-container fluid>
    <v-progress-linear v-if="loading && !thing" indeterminate />

    <template v-if="thing">
      <div class="d-flex align-center mb-2">
        <v-btn icon="mdi-arrow-left" variant="text" @click="router.push('/things')" />
        <h1 class="text-h4 ml-2">{{ thing.name }}</h1>
      </div>

      <div class="d-flex align-center ga-2 mb-3">
        <v-chip label size="small">{{ thing.thingType?.label ?? thing.thingType?.code }}</v-chip>
      </div>

      <v-alert
        v-if="isDeleted"
        class="mb-3"
        density="compact"
        type="warning"
        variant="tonal"
      >{{ t('messages.thing.deletedReadOnly') }}</v-alert>

      <div class="d-flex flex-wrap ga-2 mb-4">
        <v-btn
          v-if="editable && !editing"
          color="primary"
          prepend-icon="mdi-pencil"
          variant="tonal"
          @click="startEdit"
        >{{ t('actions.thing.edit') }}</v-btn>

        <v-btn
          v-if="editable"
          color="error"
          prepend-icon="mdi-delete"
          variant="text"
          @click="deleteOpen = true"
        >{{ t('actions.thing.delete') }}</v-btn>
      </div>

      <v-row>
        <v-col cols="12" md="8">
          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.thing.identity') }}</v-card-title>

            <v-card-text>
              <v-form v-if="editing" ref="editForm" @submit.prevent="saveEdit">
                <ThingMainForm v-model="editModel" />
                <v-text-field v-model="editReason" class="mt-2" :label="t('finalize.reason')" />

                <div class="d-flex justify-end ga-2">
                  <v-btn variant="text" @click="editing = false">{{ t('actions.common.cancel') }}</v-btn>
                  <v-btn color="primary" :loading="saving" type="submit" variant="flat">{{ t('actions.common.save') }}</v-btn>
                </div>
              </v-form>

              <v-table v-else density="compact">
                <tbody>
                  <template v-if="thing.parcel">
                    <tr><td class="text-medium-emphasis" style="width: 40%">{{ t('fields.thing.commune_ofs') }}</td><td>{{ thing.parcel.communeOfs }}</td></tr>
                    <tr><td class="text-medium-emphasis">{{ t('fields.thing.parcel_number') }}</td><td>{{ thing.parcel.parcelNumber }}</td></tr>
                    <tr><td class="text-medium-emphasis">{{ t('fields.thing.egrid') }}</td><td>{{ thing.parcel.egrid || '—' }}</td></tr>
                    <tr><td class="text-medium-emphasis">{{ t('fields.thing.surface_m2') }}</td><td>{{ thing.parcel.surfaceM2 ? `${thing.parcel.surfaceM2} m²` : '—' }}</td></tr>
                  </template>

                  <template v-if="thing.building">
                    <tr><td class="text-medium-emphasis" style="width: 40%">{{ t('fields.thing.egid') }}</td><td>{{ thing.building.egid || '—' }}</td></tr>
                    <tr><td class="text-medium-emphasis">{{ t('fields.thing.eca_number') }}</td><td>{{ thing.building.ecaNumber || '—' }}</td></tr>
                    <tr><td class="text-medium-emphasis">{{ t('fields.thing.construction_year') }}</td><td>{{ thing.building.constructionYear || '—' }}</td></tr>
                    <tr><td class="text-medium-emphasis">{{ t('fields.thing.building_status') }}</td><td>{{ enumLabel('BuildingStatus', thing.building.buildingStatus ?? 'BUILDING_STATUS_UNSPECIFIED') }}</td></tr>
                  </template>

                  <tr><td class="text-medium-emphasis" style="width: 40%">{{ t('fields.thing.description') }}</td><td>{{ thing.description || '—' }}</td></tr>
                  <tr><td class="text-medium-emphasis">{{ t('fields.thing.external_ref') }}</td><td>{{ thing.externalRef || '—' }}</td></tr>
                </tbody>
              </v-table>
            </v-card-text>
          </v-card>

          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.thing.geometry') }}</v-card-title>

            <v-card-text class="d-flex flex-wrap ga-6 align-start">
              <GeometryPreview :geojson="thing.geometryGeojson" />

              <v-table v-if="thing.geometryGeojson" density="compact">
                <tbody>
                  <tr><td class="text-medium-emphasis">{{ t('fields.thing.area') }}</td><td>{{ thing.areaM2 ? `${Math.round(thing.areaM2)} m²` : '—' }}</td></tr>
                  <tr><td class="text-medium-emphasis">{{ t('fields.thing.anchor') }}</td><td>{{ Math.round(thing.anchorE ?? 0) }} / {{ Math.round(thing.anchorN ?? 0) }}</td></tr>
                  <tr v-if="mapUrl"><td colspan="2"><a :href="mapUrl" rel="noopener noreferrer" target="_blank">{{ t('actions.thing.showOnMap') }}</a></td></tr>
                </tbody>
              </v-table>
            </v-card-text>
          </v-card>

          <v-card class="mb-4">
            <v-card-title class="d-flex align-center text-subtitle-1">
              {{ t('sections.thing.relationships') }}
              <v-spacer />

              <v-btn
                v-if="editable"
                prepend-icon="mdi-link-plus"
                size="small"
                variant="tonal"
                @click="linkOpen = true"
              >{{ t('actions.document.link') }}</v-btn>
            </v-card-title>

            <v-card-text>
              <p class="text-caption text-medium-emphasis mb-2">{{ t('messages.thing.relationshipsHint') }}</p>
              <RelationshipTable :can-unlink="editable" :relationships="relationships" @ended="reload" @unlink="doUnlink" />
            </v-card-text>
          </v-card>
        </v-col>

        <v-col cols="12" md="4">
          <SubjectIdentityCard class="mb-4" :subject="thing.subjectRef" />

          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.document.governance') }}</v-card-title>
            <v-card-text><RecordMetadataPanel :metadata="thing.recordMetadata" /></v-card-text>
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
        source-kind="SUBJECT_KIND_THING"
        @submit="doLink"
      />

      <v-dialog v-model="deleteOpen" max-width="480">
        <v-card>
          <v-card-title>{{ t('delete.title') }}</v-card-title>

          <v-card-text>
            <p class="mb-3">{{ t('messages.thing.deleteConfirm') }}</p>
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
