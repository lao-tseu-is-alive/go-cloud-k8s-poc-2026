<script setup lang="ts">
  import type { AppRole, User, UserRole } from '@/api/types'
  import type { ListSort } from '@/utils/listSort'
  import { computed, onMounted, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { grantUserRole, listAppRoles, listRoleHolders, listUserRoles, revokeUserRole } from '@/api/roleClient'
  import SortableHeader from '@/components/core/SortableHeader.vue'
  import UserLabel from '@/components/core/UserLabel.vue'
  import UserPicker from '@/components/core/UserPicker.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useAuthStore } from '@/stores/auth'
  import { useUiStore } from '@/stores/ui'
  import { formatDateTime } from '@/utils/formatters'
  import { sortRows } from '@/utils/listSort'
  import { required } from '@/utils/validation'

  // Application roles stored in Goéland (GLD-047): the holders of a role, with
  // grant and revoke (a reason each, audited on the user) and each user's
  // history. The server refuses to revoke the last administrator.
  const { t } = useI18n()
  const { report } = useApiErrors()
  const ui = useUiStore()
  const auth = useAuthStore()

  const roles = ref<AppRole[]>([])
  const roleCode = ref('ADMIN')
  const holders = ref<User[]>([])
  // Every holder of the role is loaded: sorted here (GLD-056).
  const holderSort = ref<ListSort>()
  const sortedHolders = computed(() => sortRows(holders.value, holderSort.value, (u, field) =>
    field === 'email' ? u.email : (u.displayName || u.id)))
  const loading = ref(false)

  const granting = ref(false)
  const grantUser = ref<string | undefined>()
  const revoking = ref<User | null>(null)
  const reason = ref('')
  const busy = ref(false)

  const historyOf = ref<User | null>(null)
  const history = ref<UserRole[]>([])

  const role = computed(() => roles.value.find(r => r.code === roleCode.value))
  const lastAdmin = computed(() => roleCode.value === 'ADMIN' && holders.value.length <= 1)

  async function loadHolders () {
    loading.value = true
    try {
      holders.value = await listRoleHolders(roleCode.value)
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  function openGrant () {
    grantUser.value = undefined
    reason.value = ''
    granting.value = true
  }

  function openRevoke (user: User) {
    reason.value = ''
    revoking.value = user
  }

  async function openHistory (user: User) {
    historyOf.value = user
    history.value = []
    try {
      history.value = await listUserRoles(user.id, true)
    } catch (error) {
      report(error)
    }
  }

  // change runs a grant or a revocation, then refreshes the holders and, when
  // the caller changed its own roles, its session.
  async function change (userId: string, action: () => Promise<unknown>, message: string) {
    busy.value = true
    try {
      await action()
      ui.notify(t(message), 'success')
      granting.value = false
      revoking.value = null
      await loadHolders()
      if (userId === auth.me?.id) await auth.loadMe()
    } catch (error) {
      report(error)
    } finally {
      busy.value = false
    }
  }

  function doGrant () {
    const userId = grantUser.value
    if (!userId || !reason.value.trim()) return
    void change(userId, () => grantUserRole(userId, roleCode.value, reason.value.trim()), 'roles.messages.granted')
  }

  function doRevoke () {
    const user = revoking.value
    if (!user || !reason.value.trim()) return
    void change(user.id, () => revokeUserRole(user.id, roleCode.value, reason.value.trim()), 'roles.messages.revoked')
  }

  onMounted(async () => {
    try {
      roles.value = await listAppRoles()
    } catch (error) {
      report(error)
    }
  })
  watch(roleCode, loadHolders, { immediate: true })
</script>

<template>
  <div>
    <div class="d-flex flex-wrap align-center ga-4 mb-2">
      <v-select
        v-model="roleCode"
        density="compact"
        hide-details
        item-title="label"
        item-value="code"
        :items="roles"
        :label="t('roles.fields.role')"
        max-width="320"
      />

      <span v-if="role?.description" class="text-body-2 text-medium-emphasis">{{ role.description }}</span>
      <v-spacer />

      <v-btn
        color="primary"
        :disabled="role && !role.isActive"
        prepend-icon="mdi-account-plus-outline"
        variant="flat"
        @click="openGrant"
      >
        {{ t('roles.actions.grant') }}
      </v-btn>
    </div>

    <v-progress-linear v-if="loading" class="mb-2" color="primary" indeterminate />
    <p v-if="!loading && holders.length === 0" class="text-medium-emphasis">{{ t('roles.empty') }}</p>

    <v-table v-else density="compact">
      <thead>
        <tr>
          <SortableHeader v-model:sort="holderSort" field="user" :label="t('roles.fields.user')" />
          <SortableHeader v-model:sort="holderSort" field="email" :label="t('roles.fields.email')" />
          <th scope="col" />
        </tr>
      </thead>

      <tbody>
        <tr v-for="user in sortedHolders" :key="user.id">
          <td>{{ user.displayName || user.id }}</td>
          <td class="text-medium-emphasis">{{ user.email || '—' }}</td>

          <td class="text-right">
            <v-btn prepend-icon="mdi-history" size="small" variant="text" @click="openHistory(user)">{{ t('roles.actions.history') }}</v-btn>

            <v-btn
              color="error"
              :disabled="lastAdmin"
              prepend-icon="mdi-account-remove-outline"
              size="small"
              :title="lastAdmin ? t('roles.lastAdmin') : undefined"
              variant="text"
              @click="openRevoke(user)"
            >
              {{ t('roles.actions.revoke') }}
            </v-btn>
          </td>
        </tr>
      </tbody>
    </v-table>

    <p v-if="lastAdmin && holders.length === 1" class="text-caption text-medium-emphasis mt-2">{{ t('roles.lastAdmin') }}</p>

    <v-dialog v-model="granting" max-width="520">
      <v-card :title="t('roles.grantTitle', { role: role?.label || roleCode })">
        <v-card-text>
          <UserPicker v-model="grantUser" :label="t('roles.pickUser')" />
          <p class="text-caption text-medium-emphasis mb-3">{{ t('roles.signedInOnly') }}</p>
          <v-text-field v-model="reason" :label="t('roles.fields.reason')" :rules="[required(t)]" />
        </v-card-text>

        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="granting = false">{{ t('actions.common.cancel') }}</v-btn>

          <v-btn
            color="primary"
            :disabled="!grantUser || !reason.trim()"
            :loading="busy"
            variant="flat"
            @click="doGrant"
          >{{ t('roles.actions.grant') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog max-width="520" :model-value="!!revoking" @update:model-value="revoking = null">
      <v-card :title="t('roles.revokeTitle', { role: role?.label || roleCode, user: revoking?.displayName || revoking?.id })">
        <v-card-text>
          <v-text-field v-model="reason" :label="t('roles.fields.reason')" :rules="[required(t)]" />
        </v-card-text>

        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="revoking = null">{{ t('actions.common.cancel') }}</v-btn>

          <v-btn
            color="error"
            :disabled="!reason.trim()"
            :loading="busy"
            variant="flat"
            @click="doRevoke"
          >{{ t('roles.actions.revoke') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog max-width="860" :model-value="!!historyOf" @update:model-value="historyOf = null">
      <v-card :title="t('roles.historyTitle', { user: historyOf?.displayName || historyOf?.id })">
        <v-card-text>
          <p v-if="history.length === 0" class="text-medium-emphasis">{{ t('messages.common.noData') }}</p>

          <v-table v-else density="compact">
            <thead>
              <tr>
                <th scope="col">{{ t('roles.fields.role') }}</th>
                <th scope="col">{{ t('roles.fields.granted') }}</th>
                <th scope="col">{{ t('roles.fields.revoked') }}</th>
              </tr>
            </thead>

            <tbody>
              <tr v-for="r in history" :key="r.id">
                <td><code>{{ r.roleCode }}</code></td>

                <td>
                  <div>{{ formatDateTime(r.grantedAt) }} · <UserLabel :id="r.grantedBy" /></div>
                  <div class="text-caption text-medium-emphasis">{{ r.grantReason || '—' }}</div>
                </td>

                <td>
                  <template v-if="r.revokedAt">
                    <div>{{ formatDateTime(r.revokedAt) }} · <UserLabel :id="r.revokedBy" /></div>
                    <div class="text-caption text-medium-emphasis">{{ r.revokeReason || '—' }}</div>
                  </template>

                  <v-chip v-else color="success" label size="x-small">{{ t('roles.current') }}</v-chip>
                </td>
              </tr>
            </tbody>
          </v-table>
        </v-card-text>

        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="historyOf = null">{{ t('actions.common.close') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>
