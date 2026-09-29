<script setup lang="ts">
  import type { OrgUnitInput, OrgUnitType, SubjectRef } from '@/api/types'
  import type { OrgUnitForm } from '@/components/orgunit/orgUnitForm'
  import { computed, nextTick, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { listOrgUnitTypes } from '@/api/orgUnitClient'
  import SubjectPicker from '@/components/core/SubjectPicker.vue'
  import { formToInput } from '@/components/orgunit/orgUnitForm'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { maxLength, required } from '@/utils/validation'

  // Creates a unit (optionally under a given parent) or edits one. The parent is
  // chosen with the subject picker; the server refuses a cycle or a dissolved parent.
  const props = defineProps<{ mode: 'create' | 'edit', initial: OrgUnitForm, unitId?: string, busy?: boolean }>()
  const open = defineModel<boolean>({ default: false })
  const emit = defineEmits<{ submit: [input: OrgUnitInput] }>()

  const { t } = useI18n()
  const { report } = useApiErrors()
  const EMAIL = /^[^\s@]+@[^\s@][^\s.@]*\.[^\s@]+$/

  const form = ref<{ validate: () => Promise<{ valid: boolean }> } | null>(null)
  const model = ref<OrgUnitForm>({ ...props.initial })
  const reason = ref('')
  const types = ref<OrgUnitType[]>([])
  const pickerId = ref<string | undefined>()
  const changingParent = ref(false)

  // Active types, plus the current one even when deactivated (it stays valid).
  const typeItems = computed(() => types.value
    .filter(ty => ty.isActive || ty.code === props.initial.orgUnitTypeCode)
    .map(ty => ({ value: ty.code, title: ty.label || ty.code })))
  const emailRule = (v: string) => !v || EMAIL.test(v) || t('orgUnits.fields.emailInvalid')

  watch(open, async isOpen => {
    if (!isOpen) return
    model.value = { ...props.initial }
    reason.value = ''
    changingParent.value = false
    pickerId.value = undefined
    try {
      types.value = await listOrgUnitTypes(false)
    } catch (error) {
      report(error)
    }
  })

  async function pickParent (subject: SubjectRef) {
    model.value.parentId = subject.id
    model.value.parentLabel = subject.displayLabel
    changingParent.value = false
    await nextTick()
    pickerId.value = undefined
  }

  function makeRoot () {
    model.value.parentId = undefined
    model.value.parentLabel = ''
  }

  async function submit () {
    const result = await form.value?.validate()
    if (!result?.valid) return
    emit('submit', formToInput(model.value, reason.value))
  }
</script>

<template>
  <v-dialog v-model="open" max-width="640" scrollable>
    <v-card>
      <v-card-title>{{ t(`orgUnits.dialog.${mode}`) }}</v-card-title>

      <v-card-text>
        <v-form ref="form" @submit.prevent="submit">
          <v-row dense>
            <v-col cols="12" sm="8">
              <v-text-field
                v-model="model.label"
                :hint="t('orgUnits.fields.labelHint')"
                :label="t('orgUnits.fields.label')"
                persistent-hint
                :rules="[required(t), maxLength(t, 200)]"
              />
            </v-col>

            <v-col cols="12" sm="4">
              <v-text-field
                v-model="model.abbreviation"
                :hint="t('orgUnits.fields.abbreviationHint')"
                :label="t('orgUnits.fields.abbreviation')"
                persistent-hint
                :rules="[maxLength(t, 50)]"
              />
            </v-col>
          </v-row>

          <v-select
            v-model="model.orgUnitTypeCode"
            class="mt-2"
            :items="typeItems"
            :label="t('orgUnits.fields.type')"
            :rules="[required(t)]"
          />

          <div class="text-subtitle-2 mb-1">{{ t('orgUnits.fields.parent') }}</div>

          <div class="d-flex flex-wrap align-center ga-2 mb-2">
            <v-chip v-if="model.parentId" prepend-icon="mdi-sitemap-outline" size="small">{{ model.parentLabel || model.parentId }}</v-chip>
            <span v-else class="text-medium-emphasis">{{ t('orgUnits.root') }}</span>

            <v-btn size="small" variant="text" @click="changingParent = !changingParent">{{ t('orgUnits.actions.changeParent') }}</v-btn>
            <v-btn v-if="model.parentId" size="small" variant="text" @click="makeRoot">{{ t('orgUnits.actions.makeRoot') }}</v-btn>
          </div>

          <SubjectPicker
            v-if="changingParent"
            v-model="pickerId"
            :exclude-id="unitId"
            kind="SUBJECT_KIND_ORG_UNIT"
            :label="t('orgUnits.fields.parent')"
            @picked="pickParent"
          />

          <v-text-field
            v-model="model.email"
            class="mt-2"
            :label="t('orgUnits.fields.email')"
            :rules="[emailRule, maxLength(t, 254)]"
            type="email"
          />

          <v-textarea
            v-model="model.description"
            auto-grow
            :label="t('orgUnits.fields.description')"
            rows="2"
            :rules="[maxLength(t, 2000)]"
          />

          <v-text-field v-model="reason" :label="t('finalize.reason')" />
        </v-form>
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="open = false">{{ t('actions.common.cancel') }}</v-btn>
        <v-btn color="primary" :loading="busy" variant="flat" @click="submit">{{ t('actions.common.save') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
