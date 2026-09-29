<script setup lang="ts">
  import type { AuditEvent, OrgUnit, OrgUnitInput, OrgUnitNode, SubjectRef, SubjectRelationship } from '@/api/types'
  import type { OrgUnitForm } from '@/components/orgunit/orgUnitForm'
  import { storeToRefs } from 'pinia'
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { linkSubjects, unlinkSubjects } from '@/api/coreClient'
  import { createOrgUnit, dissolveOrgUnit, getOrgUnit, updateOrgUnit } from '@/api/orgUnitClient'
  import AuditTimeline from '@/components/core/AuditTimeline.vue'
  import RecordMetadataPanel from '@/components/core/RecordMetadataPanel.vue'
  import RelationshipTable from '@/components/core/RelationshipTable.vue'
  import SubjectIdentityCard from '@/components/core/SubjectIdentityCard.vue'
  import SubjectPicker from '@/components/core/SubjectPicker.vue'
  import { emptyOrgUnitForm, forgetOrgUnitLabel, nodeLabel, orgUnitToForm } from '@/components/orgunit/orgUnitForm'
  import OrgUnitFormDialog from '@/components/orgunit/OrgUnitFormDialog.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useAuthStore } from '@/stores/auth'
  import { useUiStore } from '@/stores/ui'
  import { formatDateTime } from '@/utils/formatters'
  import { required } from '@/utils/validation'

  const { t } = useI18n()
  const route = useRoute()
  const router = useRouter()
  const { report } = useApiErrors()
  const ui = useUiStore()
  const { isAdmin } = storeToRefs(useAuthStore())

  const id = computed(() => String(route.params.id))
  const unit = ref<OrgUnit | null>(null)
  const ancestors = ref<OrgUnitNode[]>([])
  const children = ref<OrgUnitNode[]>([])
  const relationships = ref<SubjectRelationship[]>([])
  const audit = ref<AuditEvent[]>([])
  const loading = ref(true)

  const dialogOpen = ref(false)
  const dialogMode = ref<'create' | 'edit'>('edit')
  const dialogInitial = ref<OrgUnitForm>(emptyOrgUnitForm())
  const dialogBusy = ref(false)
  const dissolveOpen = ref(false)
  const dissolveReason = ref('')
  const dissolveBusy = ref(false)

  // Membership (USER_MEMBER_OF_ORG_UNIT) has its own section; the table shows the rest.
  const MEMBER_TYPE = 'USER_MEMBER_OF_ORG_UNIT'
  const members = computed(() => relationships.value.filter(r => r.relationshipType?.code === MEMBER_TYPE && !r.validTo))
  const otherRelationships = computed(() => relationships.value.filter(r => r.relationshipType?.code !== MEMBER_TYPE))
  const memberPick = ref<string | undefined>()
  const memberBusy = ref(false)

  async function addMember (user: SubjectRef) {
    memberBusy.value = true
    try {
      await linkSubjects(user.id, id.value, MEMBER_TYPE)
      ui.notify(t('orgUnits.messages.memberAdded'), 'success')
      memberPick.value = undefined
      await reload()
    } catch (error) {
      report(error)
    } finally {
      memberBusy.value = false
    }
  }

  async function removeMember (rel: SubjectRelationship) {
    try {
      await unlinkSubjects(rel.id, '')
      ui.notify(t('orgUnits.messages.memberRemoved'), 'success')
      await reload()
    } catch (error) {
      report(error)
    }
  }

  const dissolved = computed(() => !!unit.value?.dissolvedAt)
  const manageable = computed(() => isAdmin.value && !!unit.value && !dissolved.value)
  const liveChildren = computed(() => children.value.filter(c => !c.dissolved).length)
  const parentNode = computed(() => ancestors.value.at(-1))
  const breadcrumbs = computed(() => [
    { title: t('orgUnits.title'), to: '/org-units' },
    ...ancestors.value.map(a => ({ title: nodeLabel(a), to: `/org-units/${encodeURIComponent(a.id)}` })),
    { title: unit.value ? nodeLabel(unit.value) : '…', disabled: true },
  ])

  async function reload () {
    loading.value = true
    try {
      const res = await getOrgUnit(id.value, { includeRelationships: true, includeAudit: true })
      unit.value = res.orgUnit ?? null
      ancestors.value = res.ancestors ?? []
      children.value = res.children ?? []
      relationships.value = res.relationships ?? []
      audit.value = res.recentAudit ?? []
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  function openEdit () {
    if (!unit.value) return
    dialogMode.value = 'edit'
    dialogInitial.value = orgUnitToForm(unit.value, parentNode.value ? nodeLabel(parentNode.value) : '')
    dialogOpen.value = true
  }

  function openCreateChild () {
    if (!unit.value) return
    dialogMode.value = 'create'
    dialogInitial.value = emptyOrgUnitForm({ id: id.value, label: nodeLabel(unit.value) })
    dialogOpen.value = true
  }

  async function submit (input: OrgUnitInput) {
    dialogBusy.value = true
    try {
      if (dialogMode.value === 'edit') {
        await updateOrgUnit(id.value, input)
        forgetOrgUnitLabel(id.value)
        ui.notify(t('orgUnits.messages.updated'), 'success')
      } else {
        await createOrgUnit(input)
        ui.notify(t('orgUnits.messages.created'), 'success')
      }
      dialogOpen.value = false
      await reload()
    } catch (error) {
      report(error)
    } finally {
      dialogBusy.value = false
    }
  }

  async function doDissolve () {
    if (!dissolveReason.value.trim()) return
    dissolveBusy.value = true
    try {
      await dissolveOrgUnit(id.value, dissolveReason.value.trim())
      forgetOrgUnitLabel(id.value)
      ui.notify(t('orgUnits.messages.dissolved'), 'success')
      dissolveOpen.value = false
      await reload()
    } catch (error) {
      report(error)
    } finally {
      dissolveBusy.value = false
    }
  }

  function openDissolve () {
    dissolveReason.value = ''
    dissolveOpen.value = true
  }

  // Reload on id change too: following a parent or child link reuses this page.
  watch(id, reload, { immediate: true })
</script>

<template>
  <v-container fluid>
    <div class="d-flex align-center ga-2 mb-1">
      <v-btn icon="mdi-arrow-left" variant="text" @click="router.push('/org-units')" />
      <h1 class="text-h5 text-truncate">{{ unit ? unit.label : t('orgUnits.title') }}</h1>
    </div>

    <v-breadcrumbs class="pa-0 mb-2" density="compact" :items="breadcrumbs" />

    <v-progress-linear v-if="loading" color="primary" indeterminate />

    <template v-if="unit">
      <div class="d-flex flex-wrap ga-2 mb-3 align-center">
        <v-chip color="primary" label size="small">{{ unit.orgUnitType?.label ?? unit.orgUnitType?.code }}</v-chip>
        <v-chip v-if="unit.abbreviation" label size="small" variant="outlined">{{ unit.abbreviation }}</v-chip>
        <v-chip v-if="dissolved" color="grey" prepend-icon="mdi-archive-outline" size="small">{{ t('orgUnits.dissolved') }}</v-chip>
      </div>

      <v-alert
        v-if="dissolved"
        class="mb-3"
        density="compact"
        type="info"
        variant="tonal"
      >
        {{ t('orgUnits.messages.dissolvedReadOnly', { date: formatDateTime(unit.dissolvedAt), reason: unit.dissolutionReason }) }}
      </v-alert>

      <div v-if="manageable" class="d-flex flex-wrap ga-2 mb-4">
        <v-btn color="primary" prepend-icon="mdi-pencil" variant="tonal" @click="openEdit">{{ t('orgUnits.actions.edit') }}</v-btn>
        <v-btn prepend-icon="mdi-file-tree" variant="tonal" @click="openCreateChild">{{ t('orgUnits.actions.createChild') }}</v-btn>

        <v-btn
          color="error"
          :disabled="liveChildren > 0"
          prepend-icon="mdi-archive-arrow-down-outline"
          :title="liveChildren > 0 ? t('orgUnits.messages.dissolveBlocked', { n: liveChildren }) : ''"
          variant="text"
          @click="openDissolve"
        >
          {{ t('orgUnits.actions.dissolve') }}
        </v-btn>
      </div>

      <v-row>
        <v-col cols="12" md="8">
          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.case.summary') }}</v-card-title>

            <v-card-text>
              <v-table density="compact">
                <tbody>
                  <tr><td class="text-medium-emphasis" style="width: 35%">{{ t('orgUnits.fields.label') }}</td><td>{{ unit.label }}</td></tr>
                  <tr><td class="text-medium-emphasis">{{ t('orgUnits.fields.abbreviation') }}</td><td>{{ unit.abbreviation || '—' }}</td></tr>
                  <tr><td class="text-medium-emphasis">{{ t('orgUnits.fields.type') }}</td><td>{{ unit.orgUnitType?.label ?? '—' }}</td></tr>

                  <tr>
                    <td class="text-medium-emphasis">{{ t('orgUnits.fields.email') }}</td>
                    <td><a v-if="unit.email" :href="`mailto:${unit.email}`">{{ unit.email }}</a><span v-else>—</span></td>
                  </tr>

                  <tr><td class="text-medium-emphasis">{{ t('orgUnits.fields.description') }}</td><td>{{ unit.description || '—' }}</td></tr>
                  <tr><td class="text-medium-emphasis">{{ t('orgUnits.fields.externalRef') }}</td><td>{{ unit.externalRef || '—' }}</td></tr>
                </tbody>
              </v-table>
            </v-card-text>
          </v-card>

          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('orgUnits.children', { n: children.length }) }}</v-card-title>

            <v-card-text>
              <p v-if="children.length === 0" class="text-medium-emphasis">{{ t('orgUnits.noChildren') }}</p>

              <v-list v-else density="compact">
                <v-list-item
                  v-for="child in children"
                  :key="child.id"
                  prepend-icon="mdi-sitemap-outline"
                  :subtitle="child.dissolved ? t('orgUnits.dissolved') : undefined"
                  :title="nodeLabel(child)"
                  :to="`/org-units/${encodeURIComponent(child.id)}`"
                />
              </v-list>
            </v-card-text>
          </v-card>

          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('orgUnits.members', { n: members.length }) }}</v-card-title>

            <v-card-text>
              <p class="text-caption text-medium-emphasis mb-2">{{ t('orgUnits.membersHint') }}</p>
              <p v-if="members.length === 0" class="text-medium-emphasis">{{ t('orgUnits.noMembers') }}</p>

              <div v-else class="d-flex flex-wrap ga-2 mb-3">
                <v-chip
                  v-for="m in members"
                  :key="m.id"
                  :closable="manageable"
                  prepend-icon="mdi-account-circle-outline"
                  size="small"
                  @click:close="removeMember(m)"
                >
                  {{ m.source?.displayLabel }}
                </v-chip>
              </div>

              <SubjectPicker
                v-if="manageable"
                v-model="memberPick"
                :disabled="memberBusy"
                kind="SUBJECT_KIND_USER"
                :label="t('orgUnits.actions.addMember')"
                @picked="addMember"
              />
            </v-card-text>
          </v-card>

          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.case.relationships') }}</v-card-title>

            <v-card-text>
              <p class="text-caption text-medium-emphasis mb-2">{{ t('orgUnits.relationshipsHint') }}</p>
              <RelationshipTable :can-unlink="false" :relationships="otherRelationships" @ended="reload" />
            </v-card-text>
          </v-card>
        </v-col>

        <v-col cols="12" md="4">
          <SubjectIdentityCard class="mb-4" :subject="unit.subjectRef" />

          <v-card class="mb-4">
            <v-card-title class="text-subtitle-1">{{ t('sections.document.governance') }}</v-card-title>
            <v-card-text><RecordMetadataPanel :metadata="unit.recordMetadata" /></v-card-text>
          </v-card>

          <v-card>
            <v-card-title class="text-subtitle-1">{{ t('sections.document.audit') }}</v-card-title>
            <v-card-text><AuditTimeline :events="audit" /></v-card-text>
          </v-card>
        </v-col>
      </v-row>

      <OrgUnitFormDialog
        v-model="dialogOpen"
        :busy="dialogBusy"
        :initial="dialogInitial"
        :mode="dialogMode"
        :unit-id="dialogMode === 'edit' ? id : undefined"
        @submit="submit"
      />

      <v-dialog v-model="dissolveOpen" max-width="480">
        <v-card>
          <v-card-title>{{ t('orgUnits.actions.dissolve') }}</v-card-title>

          <v-card-text>
            <p class="mb-3">{{ t('orgUnits.messages.dissolveConfirm') }}</p>
            <v-text-field v-model="dissolveReason" :label="t('finalize.reason')" :rules="[required(t)]" />
          </v-card-text>

          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="dissolveOpen = false">{{ t('actions.common.cancel') }}</v-btn>
            <v-btn color="error" :loading="dissolveBusy" variant="flat" @click="doDissolve">{{ t('actions.common.confirm') }}</v-btn>
          </v-card-actions>
        </v-card>
      </v-dialog>
    </template>
  </v-container>
</template>
