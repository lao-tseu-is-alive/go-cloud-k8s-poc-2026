<script setup lang="ts">
  import type { CaseType, CaseTypeDefaultGrant, GranteeKind, Permission } from '@/api/types'
  import { ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { setCaseTypeDefaultGrants } from '@/api/caseClient'
  import { GRANTEE_ICONS, LEVEL_COLORS, LEVELS } from '@/components/access/grantees'
  import GroupPicker from '@/components/access/GroupPicker.vue'
  import SubjectPicker from '@/components/core/SubjectPicker.vue'
  import UserPicker from '@/components/core/UserPicker.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { useUiStore } from '@/stores/ui'
  import { maxLength } from '@/utils/validation'

  // The default grants of a case type (GLD-050): copied once onto every new
  // case of the type, next to its creator's FULL_CONTROL and its owning unit's
  // MANAGE. "The creator's units" is resolved when a case is created; changing
  // the template leaves existing cases unchanged. Administrators only.
  const props = defineProps<{ caseType: CaseType | null }>()
  const emit = defineEmits<{ close: [], saved: [] }>()
  const { t } = useI18n()
  const { enumLabel } = useI18nEnum()
  const { report } = useApiErrors()
  const ui = useUiStore()

  const KINDS: GranteeKind[] = ['GRANTEE_KIND_CREATOR_UNITS', 'GRANTEE_KIND_USER', 'GRANTEE_KIND_GROUP', 'GRANTEE_KIND_ORG_UNIT']
  const KIND_KEYS: Partial<Record<GranteeKind, string>> = {
    GRANTEE_KIND_CREATOR_UNITS: 'creatorUnits', GRANTEE_KIND_USER: 'user', GRANTEE_KIND_GROUP: 'group', GRANTEE_KIND_ORG_UNIT: 'unit',
  }

  const lines = ref<CaseTypeDefaultGrant[]>([])
  const kind = ref<GranteeKind>('GRANTEE_KIND_CREATOR_UNITS')
  const granteeId = ref<string | undefined>()
  const level = ref<Permission>('PERMISSION_CONTRIBUTE')
  const reason = ref('')
  const saving = ref(false)

  watch(() => props.caseType, caseType => {
    lines.value = (caseType?.defaultGrants ?? []).map(g => ({ ...g }))
    reason.value = ''
    granteeId.value = undefined
  }, { immediate: true })

  const kindLabel = (k: GranteeKind) => t(`caseTypeDefaults.kinds.${KIND_KEYS[k] ?? 'user'}`)

  function lineLabel (g: CaseTypeDefaultGrant): string {
    return g.granteeKind === 'GRANTEE_KIND_CREATOR_UNITS' ? kindLabel(g.granteeKind) : (g.granteeLabel || g.granteeId || '—')
  }

  const sameGrantee = (a: CaseTypeDefaultGrant, kindOf: GranteeKind, id?: string) => a.granteeKind === kindOf && (a.granteeId ?? '') === (id ?? '')

  function canAdd (): boolean {
    const needsId = kind.value !== 'GRANTEE_KIND_CREATOR_UNITS'
    return (!needsId || !!granteeId.value) && !lines.value.some(g => sameGrantee(g, kind.value, granteeId.value))
  }

  function add () {
    if (!canAdd()) return
    lines.value = [...lines.value, { granteeKind: kind.value, granteeId: granteeId.value, level: level.value }]
    granteeId.value = undefined
  }

  function remove (index: number) {
    lines.value = lines.value.filter((_, i) => i !== index)
  }

  async function save () {
    if (!props.caseType) return
    saving.value = true
    try {
      await setCaseTypeDefaultGrants(props.caseType.code, lines.value, reason.value.trim())
      ui.notify(t('caseTypeDefaults.saved'), 'success')
      emit('saved')
    } catch (error) {
      report(error)
    } finally {
      saving.value = false
    }
  }
</script>

<template>
  <v-dialog max-width="680" :model-value="!!caseType" @update:model-value="emit('close')">
    <v-card :title="t('caseTypeDefaults.title', { type: caseType?.label })">
      <v-card-text>
        <p class="text-body-2 mb-3">{{ t('caseTypeDefaults.hint') }}</p>

        <p v-if="lines.length === 0" class="text-medium-emphasis mb-2">{{ t('caseTypeDefaults.empty') }}</p>

        <v-list v-else class="mb-3" density="compact">
          <v-list-item v-for="(g, i) in lines" :key="g.granteeKind + (g.granteeId ?? '')" :prepend-icon="GRANTEE_ICONS[g.granteeKind]">
            <v-list-item-title class="d-flex flex-wrap align-center ga-2">
              <span>{{ lineLabel(g) }}</span>
              <v-chip :color="LEVEL_COLORS[g.level]" label size="x-small">{{ enumLabel('Permission', g.level) }}</v-chip>
            </v-list-item-title>

            <template #append>
              <v-btn
                color="error"
                icon="mdi-close"
                size="x-small"
                :title="t('caseTypeDefaults.remove')"
                variant="text"
                @click="remove(i)"
              />
            </template>
          </v-list-item>
        </v-list>

        <v-sheet border class="pa-3" rounded>
          <v-select
            v-model="kind"
            :item-title="kindLabel"
            :item-value="(k: GranteeKind) => k"
            :items="KINDS"
            :label="t('caseTypeDefaults.grantee')"
            @update:model-value="granteeId = undefined"
          />

          <UserPicker v-if="kind === 'GRANTEE_KIND_USER'" v-model="granteeId" :label="t('access.grantee.pickUser')" />
          <GroupPicker v-else-if="kind === 'GRANTEE_KIND_GROUP'" v-model="granteeId" :label="t('access.grantee.pickGroup')" />
          <SubjectPicker v-else-if="kind === 'GRANTEE_KIND_ORG_UNIT'" v-model="granteeId" kind="SUBJECT_KIND_ORG_UNIT" :label="t('access.grantee.pickUnit')" />
          <p v-else class="text-caption text-medium-emphasis mb-3">{{ t('caseTypeDefaults.creatorUnitsHint') }}</p>

          <div class="d-flex align-center ga-2">
            <v-select
              v-model="level"
              hide-details
              :item-title="(l: Permission) => enumLabel('Permission', l)"
              :item-value="(l: Permission) => l"
              :items="LEVELS"
              :label="t('access.fields.level')"
            />

            <v-btn
              color="primary"
              :disabled="!canAdd()"
              prepend-icon="mdi-plus"
              variant="tonal"
              @click="add"
            >{{ t('caseTypeDefaults.add') }}</v-btn>
          </div>
        </v-sheet>

        <v-text-field v-model="reason" class="mt-3" :label="t('fields.reference.reason')" :rules="[maxLength(t, 2000)]" />
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="emit('close')">{{ t('actions.common.cancel') }}</v-btn>
        <v-btn color="primary" :loading="saving" variant="flat" @click="save">{{ t('actions.common.save') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
