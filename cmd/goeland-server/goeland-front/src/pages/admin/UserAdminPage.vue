<script setup lang="ts">
  import type { AppRole, SubjectRelationship, User, UserRole } from '@/api/types'
  import { storeToRefs } from 'pinia'
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { useRoute } from 'vue-router'
  import { addGroupMember, removeGroupMember } from '@/api/accessClient'
  import { batchGetUsers, endRelationship, linkSubjects, listAllRelationships } from '@/api/coreClient'
  import { grantUserRole, listAppRoles, listUserRoles, revokeUserRole } from '@/api/roleClient'
  import GroupPicker from '@/components/access/GroupPicker.vue'
  import SubjectPicker from '@/components/core/SubjectPicker.vue'
  import UserLabel from '@/components/core/UserLabel.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useAuthStore } from '@/stores/auth'
  import { useUiStore } from '@/stores/ui'
  import { formatDateTime } from '@/utils/formatters'
  import { required } from '@/utils/validation'

  // An internal user as administrators manage it (GLD-093): the identity comes
  // from the directory (read only), Goéland gives the units, the security
  // groups and the application roles, each change with a reason and audited.
  const UNIT = 'USER_MEMBER_OF_ORG_UNIT'
  const GROUP = 'USER_MEMBER_OF_GROUP'

  const { t } = useI18n()
  const route = useRoute()
  const { report } = useApiErrors()
  const ui = useUiStore()
  const { isAdmin } = storeToRefs(useAuthStore())

  const id = computed(() => String(route.params.id))
  const user = ref<User | null>(null)
  const loading = ref(true)
  const units = ref<SubjectRelationship[]>([])
  const groups = ref<SubjectRelationship[]>([])
  const roles = ref<UserRole[]>([])
  const catalogue = ref<AppRole[]>([])
  const busy = ref(false)

  const newUnit = ref<string>()
  const newGroup = ref<string>()
  const newRole = ref<string>()
  const roleReason = ref('')

  // A removal (unit, group or role) waits for its reason in one dialog.
  type Removal = { kind: 'unit' | 'group' | 'role', key: string, label: string }
  const removal = ref<Removal | null>(null)
  const removalReason = ref('')

  const pending = computed(() => !!user.value && !user.value.firstSeenAt)
  const grantable = computed(() => catalogue.value.filter(r => r.isActive && !roles.value.some(h => h.roleCode === r.code)))
  const live = (r: SubjectRelationship) => !r.validTo && !r.deletedAt

  function roleLabel (code?: string): string {
    return catalogue.value.find(r => r.code === code)?.label || code || ''
  }

  async function loadMemberships () {
    const subjectId = user.value?.subjectId
    if (!subjectId) return
    const [u, g] = await Promise.all([
      listAllRelationships(subjectId, { outgoing: true, relationshipTypeCode: UNIT }),
      listAllRelationships(subjectId, { outgoing: true, relationshipTypeCode: GROUP }),
    ])
    units.value = u.filter(r => live(r))
    groups.value = g.filter(r => live(r))
  }

  async function load () {
    loading.value = true
    try {
      user.value = (await batchGetUsers([id.value]))[0] ?? null
      roles.value = await listUserRoles(id.value)
      catalogue.value = await listAppRoles()
      await loadMemberships()
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  /** Runs a change, then reloads the user and says it succeeded. */
  async function change (action: () => Promise<unknown>, message: string) {
    busy.value = true
    try {
      await action()
      ui.notify(t(message), 'success')
      await load()
      return true
    } catch (error) {
      report(error)
      return false
    } finally {
      busy.value = false
    }
  }

  async function attachUnit () {
    const unit = newUnit.value
    const subjectId = user.value?.subjectId
    if (!unit || !subjectId) return
    if (await change(() => linkSubjects(subjectId, unit, UNIT), 'userAdmin.messages.unitAdded')) newUnit.value = undefined
  }

  async function joinGroup () {
    const group = newGroup.value
    if (!group) return
    if (await change(() => addGroupMember(group, id.value), 'userAdmin.messages.groupAdded')) newGroup.value = undefined
  }

  async function grantRole () {
    const code = newRole.value
    if (!code || !roleReason.value.trim()) return
    if (await change(() => grantUserRole(id.value, code, roleReason.value.trim()), 'userAdmin.messages.roleGranted')) {
      newRole.value = undefined
      roleReason.value = ''
    }
  }

  function askRemoval (kind: Removal['kind'], key: string, label: string) {
    removal.value = { kind, key, label }
    removalReason.value = ''
  }

  async function confirmRemoval () {
    const r = removal.value
    const reason = removalReason.value.trim()
    if (!r || !reason) return
    const actions: Record<Removal['kind'], () => Promise<unknown>> = {
      unit: () => endRelationship(r.key, reason),
      group: () => removeGroupMember(r.key, id.value, reason),
      role: () => revokeUserRole(id.value, r.key, reason),
    }
    if (await change(actions[r.kind], `userAdmin.messages.${r.kind}Removed`)) removal.value = null
  }

  watch(id, () => void load(), { immediate: true })
</script>

<template>
  <v-container>
    <div class="d-flex align-center ga-1 mb-2 text-body-2 text-medium-emphasis">
      <router-link class="text-medium-emphasis" to="/admin">{{ t('pages.admin.title') }}</router-link>
      <span aria-hidden="true">›</span>
      <span>{{ t('users.tab') }}</span>
    </div>

    <v-alert v-if="!isAdmin" type="warning" variant="tonal">{{ t('messages.reference.adminOnly') }}</v-alert>
    <v-progress-linear v-else-if="loading && !user" color="primary" indeterminate />
    <v-alert v-else-if="!user" type="info" variant="tonal">{{ t('userAdmin.notFound') }}</v-alert>

    <template v-else>
      <div class="d-flex flex-wrap align-center ga-2 mb-4">
        <h1 class="text-h5">{{ user.displayName || user.id }}</h1>

        <v-chip
          v-if="user.isAdmin"
          color="warning"
          label
          prepend-icon="mdi-shield-account"
          size="small"
        >{{ t('auth.admin') }}</v-chip>

        <v-chip
          v-if="pending"
          color="info"
          label
          prepend-icon="mdi-history"
          size="small"
        >{{ t('users.pending') }}</v-chip>
      </div>

      <v-row>
        <v-col cols="12" md="5">
          <v-card class="mb-4" tag="section">
            <v-card-title class="text-subtitle-1">{{ t('userAdmin.identity') }}</v-card-title>

            <v-card-text>
              <p class="text-caption text-medium-emphasis mb-3">{{ t('userAdmin.identityHint') }}</p>

              <dl class="user-facts">
                <dt>{{ t('users.name') }}</dt>
                <dd>{{ user.displayName || '—' }}</dd>
                <dt>{{ t('users.email') }}</dt>
                <dd>{{ user.email || '—' }}</dd>
                <dt>{{ t('userAdmin.id') }}</dt>
                <dd>{{ user.id }}</dd>
                <dt>{{ t('users.firstSeen') }}</dt>
                <dd>{{ user.firstSeenAt ? formatDateTime(user.firstSeenAt) : t('users.pending') }}</dd>
                <dt>{{ t('users.lastSeen') }}</dt>
                <dd>{{ user.lastSeenAt ? formatDateTime(user.lastSeenAt) : '—' }}</dd>

                <template v-if="user.registeredBy">
                  <dt>{{ t('userAdmin.registeredBy') }}</dt>
                  <dd><UserLabel :id="user.registeredBy" /></dd>
                </template>
              </dl>
            </v-card-text>
          </v-card>

          <v-card class="mb-4" tag="section">
            <v-card-title class="text-subtitle-1">{{ t('users.roles') }}</v-card-title>

            <v-card-text>
              <p v-if="roles.length === 0" class="text-medium-emphasis">{{ t('userAdmin.noRoles') }}</p>

              <v-chip
                v-for="r in roles"
                :key="r.roleCode"
                class="mr-1 mb-1"
                closable
                :color="r.roleCode === 'ADMIN' ? 'warning' : 'primary'"
                label
                @click:close="askRemoval('role', r.roleCode, roleLabel(r.roleCode))"
              >{{ roleLabel(r.roleCode) }}</v-chip>

              <div v-if="grantable.length > 0" class="mt-3">
                <v-select
                  v-model="newRole"
                  clearable
                  density="compact"
                  item-title="label"
                  item-value="code"
                  :items="grantable"
                  :label="t('userAdmin.addRole')"
                />

                <v-text-field v-model="roleReason" density="compact" :label="t('finalize.reason')" />
                <v-btn :disabled="!newRole || !roleReason.trim()" :loading="busy" variant="tonal" @click="grantRole">{{ t('roles.actions.grant') }}</v-btn>
              </div>
            </v-card-text>
          </v-card>
        </v-col>

        <v-col cols="12" md="7">
          <v-card class="mb-4" tag="section">
            <v-card-title class="text-subtitle-1">{{ t('userAdmin.units') }}</v-card-title>

            <v-card-text>
              <p v-if="units.length === 0" class="text-medium-emphasis">{{ t('userAdmin.noUnits') }}</p>

              <v-list v-else density="compact">
                <v-list-item v-for="u in units" :key="u.id" :to="`/org-units/${u.target?.id}`">
                  <v-list-item-title>{{ u.target?.displayLabel }}</v-list-item-title>
                  <v-list-item-subtitle>{{ t('userAdmin.since', { date: formatDateTime(u.validFrom || u.createdAt) }) }}</v-list-item-subtitle>

                  <template #append>
                    <v-btn
                      :aria-label="t('userAdmin.removeUnit')"
                      icon="mdi-close"
                      size="small"
                      :title="t('userAdmin.removeUnit')"
                      variant="text"
                      @click.prevent.stop="askRemoval('unit', u.id, u.target?.displayLabel ?? '')"
                    />
                  </template>
                </v-list-item>
              </v-list>

              <div class="d-flex flex-wrap align-center ga-2 mt-2">
                <div class="picker"><SubjectPicker v-model="newUnit" kind="SUBJECT_KIND_ORG_UNIT" :label="t('userAdmin.addUnit')" /></div>
                <v-btn :disabled="!newUnit" :loading="busy" variant="tonal" @click="attachUnit">{{ t('userAdmin.attach') }}</v-btn>
              </div>
            </v-card-text>
          </v-card>

          <v-card class="mb-4" tag="section">
            <v-card-title class="text-subtitle-1">{{ t('userAdmin.groups') }}</v-card-title>

            <v-card-text>
              <p v-if="groups.length === 0" class="text-medium-emphasis">{{ t('userAdmin.noGroups') }}</p>

              <v-list v-else density="compact">
                <v-list-item v-for="g in groups" :key="g.id" :to="`/groups/${g.target?.id}`">
                  <v-list-item-title>{{ g.target?.displayLabel }}</v-list-item-title>
                  <v-list-item-subtitle>{{ t('userAdmin.since', { date: formatDateTime(g.validFrom || g.createdAt) }) }}</v-list-item-subtitle>

                  <template #append>
                    <v-btn
                      :aria-label="t('userAdmin.removeGroup')"
                      icon="mdi-close"
                      size="small"
                      :title="t('userAdmin.removeGroup')"
                      variant="text"
                      @click.prevent.stop="askRemoval('group', g.target?.id ?? '', g.target?.displayLabel ?? '')"
                    />
                  </template>
                </v-list-item>
              </v-list>

              <div class="d-flex flex-wrap align-center ga-2 mt-2">
                <div class="picker"><GroupPicker v-model="newGroup" :label="t('userAdmin.addGroup')" /></div>
                <v-btn :disabled="!newGroup" :loading="busy" variant="tonal" @click="joinGroup">{{ t('userAdmin.join') }}</v-btn>
              </div>
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>
    </template>

    <v-dialog max-width="480" :model-value="!!removal" @update:model-value="removal = null">
      <v-card :title="t('userAdmin.removeTitle', { label: removal?.label })">
        <v-card-text>
          <v-text-field v-model="removalReason" autofocus :label="t('finalize.reason')" :rules="[required(t)]" />
        </v-card-text>

        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="removal = null">{{ t('actions.common.cancel') }}</v-btn>

          <v-btn
            color="error"
            :disabled="!removalReason.trim()"
            :loading="busy"
            variant="flat"
            @click="confirmRemoval"
          >{{ t('userAdmin.remove') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-container>
</template>

<style scoped>
  .user-facts {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: 4px 16px;
  }

  .user-facts dt {
    color: rgba(var(--v-theme-on-surface), var(--v-medium-emphasis-opacity));
  }

  .user-facts dd {
    overflow-wrap: anywhere;
  }

  .picker {
    flex: 1 1 18rem;
    max-width: 28rem;
  }
</style>
