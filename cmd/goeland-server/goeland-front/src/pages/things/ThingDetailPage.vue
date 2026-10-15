<script setup lang="ts">
  import type { AuditEvent, GoThing } from '@/api/types'
  import type { RoleSection } from '@/utils/roleHeadings'
  import { computed, nextTick, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { deleteThing, getThing, updateThing } from '@/api/thingClient'
  import AccessPanel from '@/components/access/AccessPanel.vue'
  import AuditTimeline from '@/components/core/AuditTimeline.vue'
  import LinkSubjectDialog from '@/components/core/LinkSubjectDialog.vue'
  import RecordMetadataPanel from '@/components/core/RecordMetadataPanel.vue'
  import RelationshipTable from '@/components/core/RelationshipTable.vue'
  import SubjectIdentityCard from '@/components/core/SubjectIdentityCard.vue'
  import SubjectRolesPanel from '@/components/core/SubjectRolesPanel.vue'
  import LausanneMap from '@/components/thing/LausanneMap.vue'
  import { buildUpdateThingRequest, emptyThingForm, thingToForm } from '@/components/thing/thingForm'
  import ThingMainForm from '@/components/thing/ThingMainForm.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { useMyAccess } from '@/composables/useMyAccess'
  import { usePagedRelationships } from '@/composables/usePagedRelationships'
  import { useSubjectLinks } from '@/composables/useSubjectLinks'
  import { useUiStore } from '@/stores/ui'
  import { swissMapPointUrl } from '@/utils/geometry'
  import { kindIcon } from '@/utils/subjects'

  // The object page (GLD-069, the GLD-063 model): a header with the
  // identifiers of a parcel or a building and jump links with counts, then the
  // City of Lausanne's map with the object and its roles (owners, cases,
  // documents, contained objects) grouped and counted; the technical side
  // sits in its own tab.
  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const route = useRoute()
  const router = useRouter()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const id = computed(() => String(route.params.id))
  // The caller's level (GLD-048) hides what the server would refuse.
  const { access, reload: reloadAccess, canManage, hasFullControl } = useMyAccess(id)
  const thing = ref<GoThing | null>(null)
  const {
    relationships, hasMore: relHasMore, loading: relLoading, total: relTotal, capped: relCapped,
    sort: relSort, reload: reloadRelationships, loadMore: loadMoreRelationships,
  } = usePagedRelationships(id)
  const audit = ref<AuditEvent[]>([])
  const loading = ref(true)
  const tab = ref<'object' | 'governance'>('object')
  const sections = ref<RoleSection[]>([])
  const rolesKey = ref(0)
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
  const isLocked = computed(() => !!thing.value?.recordMetadata?.isLocked)
  const isConfidential = computed(() => (thing.value?.recordMetadata?.confidentialityLevel ?? 0) > 0)
  const editable = computed(() => !isLocked.value && !isDeleted.value && canManage.value)
  const mapUrl = computed(() => swissMapPointUrl(thing.value?.anchorE, thing.value?.anchorN))
  const icon = computed(() => {
    if (thing.value?.parcel) return 'mdi-vector-square'
    if (thing.value?.building) return 'mdi-home-city-outline'
    return 'mdi-map-marker-outline'
  })

  async function reload () {
    loading.value = true
    try {
      const res = await getThing(id.value, { includeAudit: true })
      thing.value = res.thing ?? null
      audit.value = res.recentAudit ?? []
      rolesKey.value++
      await reloadRelationships()
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  /** Shows the object tab and scrolls to a section of it. */
  async function goTo (anchor: string) {
    tab.value = 'object'
    await nextTick()
    document.querySelector(`#${anchor}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

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

  // Reload on id change too: following a link to another subject reuses this page.
  watch(id, () => {
    tab.value = 'object'
    void reload()
  }, { immediate: true })
</script>

<template>
  <v-container class="detail-page" fluid>
    <div class="d-flex align-center ga-1 mb-2 text-body-2 text-medium-emphasis">
      <v-btn
        :aria-label="t('nav.things')"
        icon="mdi-arrow-left"
        size="small"
        variant="text"
        @click="router.push('/things')"
      />

      <router-link class="text-medium-emphasis" to="/things">{{ t('nav.things') }}</router-link>
      <span aria-hidden="true">›</span>
      <span class="text-truncate">{{ thing?.name }}</span>
    </div>

    <v-progress-linear v-if="loading && !thing" color="primary" indeterminate />

    <template v-if="thing">
      <v-card aria-labelledby="thing-name" class="mb-4 pa-5" tag="section">
        <div class="d-flex flex-wrap align-start ga-4">
          <v-avatar color="primary" rounded="lg" size="52" variant="tonal"><v-icon :icon="icon" size="28" /></v-avatar>

          <div class="detail-page__title">
            <h1 id="thing-name" class="text-h5 font-weight-medium">{{ thing.name }}</h1>

            <div class="d-flex flex-wrap align-center ga-2 mt-1">
              <v-chip label size="small">{{ thing.thingType?.label ?? thing.thingType?.code }}</v-chip>

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
          </div>

          <div class="d-flex flex-wrap ga-2">
            <v-btn v-if="editable && !editing" prepend-icon="mdi-pencil" variant="outlined" @click="startEdit">{{ t('actions.thing.edit') }}</v-btn>

            <v-btn
              v-if="editable"
              color="primary"
              prepend-icon="mdi-link-plus"
              variant="flat"
              @click="linkOpen = true"
            >{{ t('detailPage.link') }}</v-btn>

            <v-menu v-if="editable && hasFullControl" location="bottom end">
              <template #activator="{ props: menu }">
                <v-btn v-bind="menu" :aria-label="t('detailPage.more')" icon="mdi-dots-vertical" variant="text" />
              </template>

              <v-list density="compact">
                <v-list-item base-color="error" prepend-icon="mdi-delete" :title="t('actions.thing.delete')" @click="deleteOpen = true" />
              </v-list>
            </v-menu>
          </div>
        </div>

        <div class="detail-page__facts">
          <template v-if="thing.parcel">
            <div><div class="detail-page__label">{{ t('fields.thing.parcel_number') }}</div>{{ thing.parcel.parcelNumber }} <span class="text-medium-emphasis">({{ t('fields.thing.commune_ofs') }} {{ thing.parcel.communeOfs }})</span></div>
            <div><div class="detail-page__label">{{ t('fields.thing.egrid') }}</div><span class="detail-page__mono">{{ thing.parcel.egrid || '—' }}</span></div>
            <div><div class="detail-page__label">{{ t('fields.thing.surface_m2') }}</div>{{ thing.parcel.surfaceM2 ? `${thing.parcel.surfaceM2} m²` : '—' }}</div>
          </template>

          <template v-if="thing.building">
            <div><div class="detail-page__label">{{ t('fields.thing.egid') }}</div><span class="detail-page__mono">{{ thing.building.egid || '—' }}</span></div>
            <div><div class="detail-page__label">{{ t('fields.thing.eca_number') }}</div>{{ thing.building.ecaNumber || '—' }}</div>
            <div><div class="detail-page__label">{{ t('fields.thing.construction_year') }}</div>{{ thing.building.constructionYear || '—' }}</div>
            <div><div class="detail-page__label">{{ t('fields.thing.building_status') }}</div>{{ enumLabel('BuildingStatus', thing.building.buildingStatus ?? 'BUILDING_STATUS_UNSPECIFIED') }}</div>
          </template>

          <div>
            <div class="detail-page__label">{{ t('fields.thing.area') }}</div>
            {{ thing.areaM2 ? `${Math.round(thing.areaM2)} m²` : '—' }}
          </div>

          <div v-if="thing.anchorE">
            <div class="detail-page__label">{{ t('fields.thing.anchor') }}</div>
            <a v-if="mapUrl" :href="mapUrl" rel="noopener noreferrer" target="_blank">{{ Math.round(thing.anchorE ?? 0) }} / {{ Math.round(thing.anchorN ?? 0) }}<v-icon class="ml-1" icon="mdi-open-in-new" size="x-small" /></a>
          </div>
        </div>

        <nav v-if="sections.length > 0" aria-labelledby="thing-jump" class="mt-4">
          <div id="thing-jump" class="detail-page__label mb-1">{{ t('detailPage.jump') }}</div>

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
        v-if="isDeleted"
        class="mb-3"
        density="compact"
        type="warning"
        variant="tonal"
      >{{ t('messages.thing.deletedReadOnly') }}</v-alert>

      <v-card v-if="editing" class="mb-4">
        <v-card-title class="text-subtitle-1">{{ t('actions.thing.edit') }}</v-card-title>

        <v-card-text>
          <v-form ref="editForm" @submit.prevent="saveEdit">
            <ThingMainForm v-model="editModel" />
            <v-text-field v-model="editReason" class="mt-2" :label="t('finalize.reason')" />

            <div class="d-flex justify-end ga-2">
              <v-btn variant="text" @click="editing = false">{{ t('actions.common.cancel') }}</v-btn>
              <v-btn color="primary" :loading="saving" type="submit" variant="flat">{{ t('actions.common.save') }}</v-btn>
            </div>
          </v-form>
        </v-card-text>
      </v-card>

      <v-tabs v-model="tab" class="mb-4" color="primary">
        <v-tab value="object">{{ t('thingPage.tabs.object') }}</v-tab>
        <v-tab value="governance">{{ t('detailPage.tabs.governance') }}</v-tab>
      </v-tabs>

      <v-window v-model="tab">
        <v-window-item value="object">
          <v-card class="mb-4">
            <v-card-text><LausanneMap :key="id" :geojson="thing.geometryGeojson" /></v-card-text>
          </v-card>

          <v-row>
            <v-col cols="12" md="8">
              <SubjectRolesPanel :reload-key="rolesKey" :subject-id="id" subject-kind="SUBJECT_KIND_THING" @sections="sections = $event" />
            </v-col>

            <v-col cols="12" md="4">
              <v-card>
                <v-card-title class="text-subtitle-1">{{ t('sections.thing.identity') }}</v-card-title>

                <v-card-text>
                  <v-table density="compact">
                    <tbody>
                      <tr><td class="text-medium-emphasis">{{ t('fields.thing.description') }}</td><td>{{ thing.description || '—' }}</td></tr>
                      <tr><td class="text-medium-emphasis">{{ t('fields.thing.external_ref') }}</td><td>{{ thing.externalRef || '—' }}</td></tr>
                    </tbody>
                  </v-table>
                </v-card-text>
              </v-card>
            </v-col>
          </v-row>
        </v-window-item>

        <v-window-item value="governance">
          <v-row>
            <v-col cols="12" md="6">
              <SubjectIdentityCard class="mb-4" :subject="thing.subjectRef" />

              <v-card class="mb-4">
                <v-card-title class="text-subtitle-1">{{ t('access.title') }}</v-card-title>
                <v-card-text><AccessPanel :access="access" :subject-id="id" @changed="reloadAccess(); reload()" /></v-card-text>
              </v-card>
            </v-col>

            <v-col cols="12" md="6">
              <v-card class="mb-4">
                <v-card-title class="text-subtitle-1">{{ t('sections.document.governance') }}</v-card-title>
                <v-card-text><RecordMetadataPanel :metadata="thing.recordMetadata" /></v-card-text>
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
