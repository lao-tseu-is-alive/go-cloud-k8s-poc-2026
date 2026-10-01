<script setup lang="ts">
  import type { Access, AccessGrant, GranteeKind, Permission } from '@/api/types'
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { listGrants, revokeGrant, setGrant } from '@/api/accessClient'
  import { GRANTEE_ICONS, LEVEL_COLORS, LEVELS } from '@/components/access/grantees'
  import GroupPicker from '@/components/access/GroupPicker.vue'
  import SubjectPicker from '@/components/core/SubjectPicker.vue'
  import UserLabel from '@/components/core/UserLabel.vue'
  import UserPicker from '@/components/core/UserPicker.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { LEVEL_RANK } from '@/composables/useMyAccess'
  import { useUiStore } from '@/stores/ui'
  import { formatDateTime } from '@/utils/formatters'
  import { required } from '@/utils/validation'

  // The grants of a subject (GLD-048): who may read, contribute, manage or
  // fully control it, and the caller's own level and its source. A holder of
  // FULL_CONTROL gives, changes and revokes grants (a reason each, audited); the
  // last FULL_CONTROL is kept by the server.
  const props = defineProps<{ subjectId: string, access?: Access }>()
  const emit = defineEmits<{ changed: [] }>()
  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const grants = ref<AccessGrant[]>([])
  const showHistory = ref(false)
  const loading = ref(false)

  const editing = ref(false)
  const granteeKind = ref<GranteeKind>('GRANTEE_KIND_USER')
  const granteeId = ref<string | undefined>()
  const level = ref<Permission>('PERMISSION_READ')
  const reason = ref('')
  const revoking = ref<AccessGrant | null>(null)
  const busy = ref(false)

  const canGrant = computed(() => LEVEL_RANK[props.access?.level ?? 'PERMISSION_NONE'] >= LEVEL_RANK.PERMISSION_FULL_CONTROL)

  async function load () {
    loading.value = true
    try {
      grants.value = await listGrants(props.subjectId, showHistory.value)
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  function openGrant (from?: AccessGrant) {
    granteeKind.value = from?.granteeKind ?? 'GRANTEE_KIND_USER'
    granteeId.value = from?.granteeId
    level.value = from?.level ?? 'PERMISSION_READ'
    reason.value = ''
    editing.value = true
  }

  function openRevoke (grant: AccessGrant) {
    reason.value = ''
    revoking.value = grant
  }

  async function run (action: () => Promise<unknown>, message: string) {
    busy.value = true
    try {
      await action()
      ui.notify(t(message), 'success')
      editing.value = false
      revoking.value = null
      await load()
      emit('changed')
    } catch (error) {
      report(error)
    } finally {
      busy.value = false
    }
  }

  function doGrant () {
    const id = granteeId.value
    if (!id || !reason.value.trim()) return
    void run(() => setGrant(props.subjectId, granteeKind.value, id, level.value, reason.value.trim()), 'access.messages.granted')
  }

  function doRevoke () {
    const grant = revoking.value
    if (!grant || !reason.value.trim()) return
    void run(() => revokeGrant(grant.id, reason.value.trim()), 'access.messages.revoked')
  }

  watch([() => props.subjectId, showHistory], load, { immediate: true })
</script>

<template>
  <div>
    <div v-if="access" class="d-flex flex-wrap align-center ga-2 mb-3">
      <span class="text-body-2">{{ t('access.mine') }}</span>
      <v-chip :color="LEVEL_COLORS[access.level ?? 'PERMISSION_NONE']" label size="small">{{ enumLabel('Permission', access.level) }}</v-chip>
      <span class="text-caption text-medium-emphasis">{{ enumLabel('AccessSource', access.source) }}</span>

      <v-chip
        v-if="access.confidential"
        color="error"
        prepend-icon="mdi-lock-outline"
        size="x-small"
        variant="outlined"
      >{{ t('access.confidential') }}</v-chip>
    </div>

    <div class="d-flex align-center mb-1">
      <v-switch
        v-model="showHistory"
        color="primary"
        density="compact"
        hide-details
        :label="t('access.history')"
      />

      <v-spacer />

      <v-btn
        v-if="canGrant"
        color="primary"
        prepend-icon="mdi-shield-plus-outline"
        size="small"
        variant="tonal"
        @click="openGrant()"
      >
        {{ t('access.actions.grant') }}
      </v-btn>
    </div>

    <v-progress-linear v-if="loading" class="mb-2" color="primary" indeterminate />

    <v-list density="compact">
      <v-list-item v-for="g in grants" :key="g.id" :class="{ 'text-disabled': g.revokedAt }" :prepend-icon="GRANTEE_ICONS[g.granteeKind ?? 'GRANTEE_KIND_UNSPECIFIED']">
        <v-list-item-title class="d-flex flex-wrap align-center ga-2">
          <span>{{ g.granteeLabel || g.granteeId }}</span>
          <v-chip :color="LEVEL_COLORS[g.level ?? 'PERMISSION_NONE']" label size="x-small">{{ enumLabel('Permission', g.level) }}</v-chip>
          <v-chip v-if="g.revokedAt" size="x-small" variant="outlined">{{ t('access.revoked') }}</v-chip>
        </v-list-item-title>

        <v-list-item-subtitle>
          {{ formatDateTime(g.grantedAt) }} · <UserLabel :id="g.grantedBy" /><template v-if="g.grantReason"> · {{ g.grantReason }}</template>
          <template v-if="g.revokedAt"><br>{{ t('access.revokedOn', { date: formatDateTime(g.revokedAt) }) }} · <UserLabel :id="g.revokedBy" /> · {{ g.revokeReason }}</template>
        </v-list-item-subtitle>

        <template v-if="canGrant && !g.revokedAt" #append>
          <v-btn
            icon="mdi-pencil-outline"
            size="x-small"
            :title="t('access.actions.change')"
            variant="text"
            @click="openGrant(g)"
          />

          <v-btn
            color="error"
            icon="mdi-close"
            size="x-small"
            :title="t('access.actions.revoke')"
            variant="text"
            @click="openRevoke(g)"
          />
        </template>
      </v-list-item>
    </v-list>

    <v-dialog v-model="editing" max-width="520">
      <v-card :title="t('access.actions.grant')">
        <v-card-text>
          <v-btn-toggle
            v-model="granteeKind"
            class="mb-3"
            color="primary"
            density="compact"
            mandatory
            variant="outlined"
            @update:model-value="granteeId = undefined"
          >
            <v-btn prepend-icon="mdi-account-circle-outline" value="GRANTEE_KIND_USER">{{ t('access.grantee.user') }}</v-btn>
            <v-btn prepend-icon="mdi-account-group-outline" value="GRANTEE_KIND_GROUP">{{ t('access.grantee.group') }}</v-btn>
            <v-btn prepend-icon="mdi-sitemap-outline" value="GRANTEE_KIND_ORG_UNIT">{{ t('access.grantee.unit') }}</v-btn>
          </v-btn-toggle>

          <UserPicker v-if="granteeKind === 'GRANTEE_KIND_USER'" v-model="granteeId" :label="t('access.grantee.pickUser')" />
          <GroupPicker v-else-if="granteeKind === 'GRANTEE_KIND_GROUP'" v-model="granteeId" :label="t('access.grantee.pickGroup')" />
          <SubjectPicker v-else v-model="granteeId" kind="SUBJECT_KIND_ORG_UNIT" :label="t('access.grantee.pickUnit')" />
          <p v-if="granteeKind === 'GRANTEE_KIND_ORG_UNIT'" class="text-caption text-medium-emphasis mb-2">{{ t('access.grantee.unitHint') }}</p>

          <v-select
            v-model="level"
            :item-title="(l: Permission) => enumLabel('Permission', l)"
            :item-value="(l: Permission) => l"
            :items="LEVELS"
            :label="t('access.fields.level')"
          />

          <v-text-field v-model="reason" :label="t('access.fields.reason')" :rules="[required(t)]" />
        </v-card-text>

        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="editing = false">{{ t('actions.common.cancel') }}</v-btn>

          <v-btn
            color="primary"
            :disabled="!granteeId || !reason.trim()"
            :loading="busy"
            variant="flat"
            @click="doGrant"
          >
            {{ t('access.actions.grant') }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog max-width="520" :model-value="!!revoking" @update:model-value="revoking = null">
      <v-card :title="t('access.revokeTitle', { grantee: revoking?.granteeLabel || revoking?.granteeId })">
        <v-card-text>
          <v-text-field v-model="reason" :label="t('access.fields.reason')" :rules="[required(t)]" />
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
          >
            {{ t('access.actions.revoke') }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>
