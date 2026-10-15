<script setup lang="ts">
  import type { ActorContact, AuditEvent, ContactType, GoActor } from '@/api/types'
  import type { RoleSection } from '@/utils/roleHeadings'
  import { computed, nextTick, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute, useRouter } from 'vue-router'
  import { deleteActor, getActor, listOrganizationCategories, updateActor } from '@/api/actorClient'
  import AccessPanel from '@/components/access/AccessPanel.vue'
  import ActorAddressesPanel from '@/components/actor/ActorAddressesPanel.vue'
  import ActorContactsPanel from '@/components/actor/ActorContactsPanel.vue'
  import { actorToForm, buildUpdateRequest, emptyActorForm } from '@/components/actor/actorForm'
  import ActorMainForm from '@/components/actor/ActorMainForm.vue'
  import AuditTimeline from '@/components/core/AuditTimeline.vue'
  import LinkSubjectDialog from '@/components/core/LinkSubjectDialog.vue'
  import RecordMetadataPanel from '@/components/core/RecordMetadataPanel.vue'
  import RelationshipTable from '@/components/core/RelationshipTable.vue'
  import SubjectIdentityCard from '@/components/core/SubjectIdentityCard.vue'
  import SubjectRolesPanel from '@/components/core/SubjectRolesPanel.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { useMyAccess } from '@/composables/useMyAccess'
  import { usePagedRelationships } from '@/composables/usePagedRelationships'
  import { useSubjectLinks } from '@/composables/useSubjectLinks'
  import { useUiStore } from '@/stores/ui'
  import { addressLines } from '@/utils/address'
  import { contactHref, formatContactValue, phoneDirectoryHref } from '@/utils/contactRules'
  import { formatDateTime } from '@/utils/formatters'
  import { kindIcon } from '@/utils/subjects'

  // The actor page (GLD-063 pilot): a header with the essentials (identity,
  // registers, main contacts, jump links with counts), then the actor's roles
  // organized by kind and role, read from the actor's point of view; the
  // technical side (access, governance, audit, the raw relationship table with
  // its end and unlink actions) sits in its own tab.
  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const route = useRoute()
  const router = useRouter()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const id = computed(() => String(route.params.id))
  // The caller's level (GLD-048) hides what the server would refuse.
  const { access, reload: reloadAccess, canManage, hasFullControl } = useMyAccess(id)
  const actor = ref<GoActor | null>(null)
  const {
    relationships, hasMore: relHasMore, loading: relLoading, total: relTotal, capped: relCapped,
    sort: relSort, reload: reloadRelationships, loadMore: loadMoreRelationships,
  } = usePagedRelationships(id)
  const audit = ref<AuditEvent[]>([])
  const loading = ref(true)
  const tab = ref<'roles' | 'governance'>('roles')
  const sections = ref<RoleSection[]>([])
  const rolesKey = ref(0)
  const categories = ref<Record<string, string>>({})

  const editing = ref(false)
  const editModel = ref(emptyActorForm())
  const editReason = ref('')
  const editForm = ref()
  const saving = ref(false)
  const togglingActive = ref(false)

  const deleteOpen = ref(false)
  const deleteReason = ref('')
  const deleteBusy = ref(false)

  const isLocked = computed(() => !!actor.value?.recordMetadata?.isLocked)
  const isDeleted = computed(() => !!actor.value?.recordMetadata?.deletedAt)
  const isActive = computed(() => !!actor.value?.isActive)
  const editable = computed(() => !isLocked.value && !isDeleted.value && canManage.value)
  const { linkOpen, linkBusy, doLink, doUnlink } = useSubjectLinks(id, reload)
  const isOrganization = computed(() => actor.value?.actorKind === 'ACTOR_KIND_ORGANIZATION')
  const isConfidential = computed(() => (actor.value?.recordMetadata?.confidentialityLevel ?? 0) > 0)
  const categoryLabel = computed(() => {
    const code = actor.value?.organization?.categoryCode
    return code ? (categories.value[code] ?? code) : ''
  })

  const PHONE_TYPES = new Set<ContactType>(['CONTACT_TYPE_PHONE', 'CONTACT_TYPE_PHONE_PRO', 'CONTACT_TYPE_MOBILE', 'CONTACT_TYPE_PHONE_PRIVATE'])

  /** The preferred contact among those whose type passes keep: the primary one, else the first. */
  function preferred (keep: (c: ActorContact) => boolean): ActorContact | undefined {
    const matching = (actor.value?.contacts ?? []).filter(c => keep(c))
    return matching.find(c => c.isPrimary) ?? matching[0]
  }

  const ide = computed(() => preferred(c => c.contactType === 'CONTACT_TYPE_IDE_FEDERAL'))
  const abacus = computed(() => preferred(c => c.contactType === 'CONTACT_TYPE_ABACUS_DEBTOR'))
  const phone = computed(() => preferred(c => !!c.contactType && PHONE_TYPES.has(c.contactType)))
  const email = computed(() => preferred(c => c.contactType === 'CONTACT_TYPE_EMAIL'))
  const website = computed(() => preferred(c => c.contactType === 'CONTACT_TYPE_WEBSITE'))
  const principalAddress = computed(() => {
    const addresses = actor.value?.addresses ?? []
    return addresses.find(a => a.isPrincipal) ?? addresses[0]
  })

  async function reload () {
    loading.value = true
    try {
      const res = await getActor(id.value, { includeAudit: true })
      actor.value = res.actor ?? null
      audit.value = res.recentAudit ?? []
      rolesKey.value++
      await reloadRelationships()
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  async function loadCategories () {
    try {
      const list = await listOrganizationCategories(false)
      categories.value = Object.fromEntries(list.map(c => [c.code, c.label]))
    } catch {
      categories.value = {} // the code is shown instead
    }
  }

  /** Shows the roles tab and scrolls to a section of it. */
  async function goTo (anchor: string) {
    tab.value = 'roles'
    await nextTick()
    document.querySelector(`#${anchor}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  function startEdit () {
    if (!actor.value) return
    editModel.value = actorToForm(actor.value)
    editReason.value = ''
    editing.value = true
  }

  async function saveEdit () {
    const validation = await editForm.value?.validate()
    if (validation && !validation.valid) return
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

  // Reload on id change too: following a link to another subject reuses this page.
  watch(id, () => {
    tab.value = 'roles'
    void reload()
  }, { immediate: true })
  void loadCategories()
</script>

<template>
  <v-container class="detail-page" fluid>
    <div class="d-flex align-center ga-1 mb-2 text-body-2 text-medium-emphasis">
      <v-btn
        :aria-label="t('pages.actors.list.title')"
        icon="mdi-arrow-left"
        size="small"
        variant="text"
        @click="router.push('/actors')"
      />

      <router-link class="text-medium-emphasis" to="/actors">{{ t('nav.actors') }}</router-link>
      <span aria-hidden="true">›</span>
      <span class="text-truncate">{{ actor?.displayName ?? t('pages.actors.detail.title') }}</span>
    </div>

    <v-progress-linear v-if="loading && !actor" color="primary" indeterminate />

    <template v-if="actor">
      <v-card aria-labelledby="actor-name" class="mb-4 pa-5" tag="section">
        <div class="d-flex flex-wrap align-start ga-4">
          <v-avatar color="primary" rounded="lg" size="52" variant="tonal">
            <v-icon :icon="isOrganization ? 'mdi-domain' : 'mdi-account'" size="28" />
          </v-avatar>

          <div class="detail-page__title">
            <h1 id="actor-name" class="text-h5 font-weight-medium">{{ actor.displayName }}</h1>

            <div class="d-flex flex-wrap align-center ga-2 mt-1">
              <v-chip label size="small">{{ enumLabel('ActorKind', actor.actorKind) }}</v-chip>
              <v-chip v-if="categoryLabel" label size="small">{{ categoryLabel }}</v-chip>
              <v-chip :color="isActive ? 'success' : 'grey'" label size="small">{{ isActive ? t('states.active') : t('states.inactive') }}</v-chip>

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

              <a
                v-if="ide"
                class="detail-page__register"
                :href="contactHref(ide.contactType, ide.value)"
                rel="noopener noreferrer"
                target="_blank"
                :title="t('actorPage.header.ideHint')"
              >{{ t('actorPage.header.ide', { value: ide.value }) }}<v-icon icon="mdi-open-in-new" size="x-small" /><span class="sr-only"> ({{ t('detailPage.newTab') }})</span></a>

              <a
                v-if="abacus"
                class="detail-page__register"
                :href="contactHref(abacus.contactType, abacus.value)"
                rel="noopener noreferrer"
                target="_blank"
                :title="t('actorPage.header.abacusHint')"
              >{{ t('actorPage.header.abacus', { value: abacus.value }) }}<v-icon icon="mdi-open-in-new" size="x-small" /><span class="sr-only"> ({{ t('detailPage.newTab') }})</span></a>
            </div>
          </div>

          <div class="d-flex flex-wrap ga-2">
            <v-btn v-if="editable && !editing" prepend-icon="mdi-pencil" variant="outlined" @click="startEdit">{{ t('actions.actor.edit') }}</v-btn>

            <v-btn
              v-if="editable"
              color="primary"
              prepend-icon="mdi-link-plus"
              variant="flat"
              @click="linkOpen = true"
            >{{ t('detailPage.link') }}</v-btn>

            <v-menu v-if="editable || (!isDeleted && !isLocked && hasFullControl)" location="bottom end">
              <template #activator="{ props: menu }">
                <v-btn v-bind="menu" :aria-label="t('detailPage.more')" icon="mdi-dots-vertical" variant="text" />
              </template>

              <v-list density="compact">
                <v-list-item
                  v-if="editable"
                  :disabled="togglingActive"
                  prepend-icon="mdi-account-switch"
                  :title="isActive ? t('actions.actor.deactivate') : t('actions.actor.activate')"
                  @click="toggleActive"
                />

                <v-list-item
                  v-if="!isDeleted && !isLocked && hasFullControl"
                  base-color="error"
                  prepend-icon="mdi-delete"
                  :title="t('actions.actor.delete')"
                  @click="deleteOpen = true"
                />
              </v-list>
            </v-menu>
          </div>
        </div>

        <div class="detail-page__facts">
          <div>
            <div class="detail-page__label">{{ t('actorPage.header.address') }}</div>

            <div v-if="principalAddress">
              <div v-for="(line, i) in addressLines(principalAddress)" :key="i">{{ line }}</div>
            </div>

            <div v-else class="text-medium-emphasis">—</div>
          </div>

          <div>
            <div class="detail-page__label">{{ t('actorPage.header.phone') }}</div>

            <div v-if="phone" class="d-flex flex-wrap align-baseline ga-2">
              <a :href="contactHref(phone.contactType, phone.value)">{{ formatContactValue(phone.contactType, phone.value) }}</a>

              <a
                v-if="phoneDirectoryHref(phone.value)"
                class="detail-page__directory"
                :href="phoneDirectoryHref(phone.value)"
                rel="noopener noreferrer"
                target="_blank"
                :title="t('actorPage.header.phoneDirectory')"
              >CH<span class="sr-only"> — {{ t('actorPage.header.phoneDirectory') }}</span></a>
            </div>

            <div v-else class="text-medium-emphasis">—</div>
          </div>

          <div>
            <div class="detail-page__label">{{ t('actorPage.header.email') }}</div>
            <a v-if="email" :href="contactHref(email.contactType, email.value)">{{ email.value }}</a>
            <div v-else class="text-medium-emphasis">—</div>
          </div>

          <div>
            <div class="detail-page__label">{{ t('actorPage.header.website') }}</div>
            <a v-if="website" :href="contactHref(website.contactType, website.value)" rel="noopener noreferrer" target="_blank">{{ website.value.replace(/^https?:\/\//, '') }}</a>
            <div v-else class="text-medium-emphasis">—</div>
          </div>
        </div>

        <nav v-if="sections.length > 0" aria-labelledby="actor-jump" class="mt-4">
          <div id="actor-jump" class="detail-page__label mb-1">{{ t('detailPage.jump') }}</div>

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
      >{{ t('messages.actor.lockedCannotEdit') }}</v-alert>

      <v-alert
        v-if="isDeleted"
        class="mb-3"
        density="compact"
        type="warning"
        variant="tonal"
      >{{ t('messages.actor.deletedReadOnly') }}</v-alert>

      <v-card v-if="editing" class="mb-4">
        <v-card-title class="text-subtitle-1">{{ t('actions.actor.edit') }}</v-card-title>

        <v-card-text>
          <v-form ref="editForm" @submit.prevent="saveEdit">
            <ActorMainForm v-model="editModel" lock-kind />
            <v-text-field v-model="editReason" class="mt-2" :label="t('delete.reason')" />

            <div class="d-flex justify-end ga-2">
              <v-btn variant="text" @click="editing = false">{{ t('actions.common.cancel') }}</v-btn>
              <v-btn color="primary" :loading="saving" type="submit" variant="flat">{{ t('actions.common.save') }}</v-btn>
            </div>
          </v-form>
        </v-card-text>
      </v-card>

      <v-tabs v-model="tab" class="mb-4" color="primary">
        <v-tab value="roles">{{ t('actorPage.tabs.roles') }}</v-tab>
        <v-tab value="governance">{{ t('detailPage.tabs.governance') }}</v-tab>
      </v-tabs>

      <v-window v-model="tab">
        <v-window-item value="roles">
          <v-row>
            <v-col cols="12" md="8">
              <SubjectRolesPanel :reload-key="rolesKey" :subject-id="id" subject-kind="SUBJECT_KIND_ACTOR" @sections="sections = $event" />
            </v-col>

            <v-col cols="12" md="4">
              <v-card class="mb-4">
                <v-card-title class="text-subtitle-1">{{ t('sections.actor.identity') }}</v-card-title>

                <v-card-text>
                  <v-table density="compact">
                    <tbody>
                      <template v-if="isOrganization">
                        <tr><td class="text-medium-emphasis">{{ t('fields.actor.legal_name') }}</td><td>{{ actor.organization?.legalName || '—' }}</td></tr>
                        <tr><td class="text-medium-emphasis">{{ t('fields.actor.org_complement') }}</td><td>{{ actor.organization?.complement || '—' }}</td></tr>
                      </template>

                      <template v-else>
                        <tr><td class="text-medium-emphasis">{{ t('fields.actor.salutation') }}</td><td>{{ enumLabel('Salutation', actor.person?.salutation ?? 'SALUTATION_UNSPECIFIED') }}</td></tr>
                        <tr><td class="text-medium-emphasis">{{ t('fields.actor.first_name') }}</td><td>{{ actor.person?.firstName || '—' }}</td></tr>
                        <tr><td class="text-medium-emphasis">{{ t('fields.actor.last_name') }}</td><td>{{ actor.person?.lastName || '—' }}</td></tr>
                        <tr v-if="actor.person?.isChRegister"><td class="text-medium-emphasis">{{ t('fields.actor.ch_register_ref') }}</td><td>{{ actor.person?.chRegisterRef || '—' }}</td></tr>
                      </template>

                      <tr><td class="text-medium-emphasis">{{ t('fields.actor.publication_code') }}</td><td>{{ actor.publicationCode || '—' }}</td></tr>
                    </tbody>
                  </v-table>
                </v-card-text>
              </v-card>

              <v-card class="mb-4">
                <v-card-title class="text-subtitle-1">{{ t('sections.actor.addresses') }}</v-card-title>
                <v-card-text><ActorAddressesPanel :addresses="actor.addresses" /></v-card-text>
              </v-card>

              <v-card class="mb-4">
                <v-card-title class="text-subtitle-1">{{ t('sections.actor.contacts') }}</v-card-title>
                <v-card-text><ActorContactsPanel :contacts="actor.contacts" /></v-card-text>
              </v-card>

              <v-card>
                <v-card-title class="text-subtitle-1">{{ t('actorPage.brief.title') }}</v-card-title>

                <v-card-text>
                  <div class="d-flex justify-space-between ga-3 mb-2"><span class="text-medium-emphasis">{{ t('actorPage.brief.confidentiality') }}</span><span>{{ actor.recordMetadata?.confidentialityLevel ?? 0 }}</span></div>
                  <div class="d-flex justify-space-between ga-3 mb-2"><span class="text-medium-emphasis">{{ t('actorPage.brief.myAccess') }}</span><span>{{ enumLabel('Permission', access?.level) }}</span></div>
                  <div class="d-flex justify-space-between ga-3 mb-3"><span class="text-medium-emphasis">{{ t('actorPage.brief.updated') }}</span><span>{{ formatDateTime(actor.updatedAt) }}</span></div>
                  <v-btn block variant="outlined" @click="tab = 'governance'">{{ t('detailPage.tabs.governance') }}</v-btn>
                </v-card-text>
              </v-card>
            </v-col>
          </v-row>
        </v-window-item>

        <v-window-item value="governance">
          <v-row>
            <v-col cols="12" md="6">
              <SubjectIdentityCard class="mb-4" :subject="actor.subjectRef" />

              <v-card class="mb-4">
                <v-card-title class="text-subtitle-1">{{ t('access.title') }}</v-card-title>
                <v-card-text><AccessPanel :access="access" :subject-id="id" @changed="reloadAccess(); reload()" /></v-card-text>
              </v-card>
            </v-col>

            <v-col cols="12" md="6">
              <v-card class="mb-4">
                <v-card-title class="text-subtitle-1">{{ t('sections.actor.governance') }}</v-card-title>
                <v-card-text><RecordMetadataPanel :metadata="actor.recordMetadata" /></v-card-text>
              </v-card>

              <v-card class="mb-4">
                <v-card-title class="text-subtitle-1">{{ t('sections.actor.audit') }}</v-card-title>
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

      <v-dialog v-model="deleteOpen" max-width="480">
        <v-card>
          <v-card-title>{{ t('actions.actor.delete') }}</v-card-title>

          <v-card-text>
            <v-alert class="mb-3" density="compact" type="warning" variant="tonal">{{ t('messages.actor.deleteConfirm') }}</v-alert>
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

    <LinkSubjectDialog
      v-model="linkOpen"
      :busy="linkBusy"
      :source-id="id"
      source-kind="SUBJECT_KIND_ACTOR"
      @submit="doLink"
    />
  </v-container>
</template>
