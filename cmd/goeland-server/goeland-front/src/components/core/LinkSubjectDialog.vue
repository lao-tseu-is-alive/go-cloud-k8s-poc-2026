<script setup lang="ts">
  import type { RelationshipType, SubjectKind } from '@/api/types'
  import { ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import RelationshipTypeSelect from './RelationshipTypeSelect.vue'
  import SubjectPicker from './SubjectPicker.vue'

  // Collects the inputs for a typed link. It emits a `submit` with the payload;
  // the parent owns the actual API call so this stays a pure input component.
  const props = defineProps<{ sourceKind?: SubjectKind, sourceId?: string, busy?: boolean }>()
  const emit = defineEmits<{ submit: [payload: { targetSubjectId: string, relationshipTypeCode: string, roleDetail: string }] }>()
  const open = defineModel<boolean>({ required: true })

  const { t } = useI18n()
  const targetSubjectId = ref<string | undefined>('')
  const relationshipTypeCode = ref<string | undefined>(undefined)
  const roleDetail = ref('')
  const targetKind = ref<SubjectKind>()

  function onTypeSelected (rt: RelationshipType | undefined) {
    targetKind.value = rt?.targetKind
    targetSubjectId.value = ''
  }

  watch(open, isOpen => {
    if (isOpen) {
      targetSubjectId.value = ''
      relationshipTypeCode.value = undefined
      roleDetail.value = ''
      targetKind.value = undefined
    }
  })

  function submit () {
    if (!targetSubjectId.value || !relationshipTypeCode.value) return
    emit('submit', {
      targetSubjectId: (targetSubjectId.value ?? '').trim(),
      relationshipTypeCode: relationshipTypeCode.value,
      roleDetail: roleDetail.value.trim(),
    })
  }
</script>

<template>
  <v-dialog v-model="open" max-width="560">
    <v-card>
      <v-card-title>{{ t('link.title') }}</v-card-title>

      <v-card-text>
        <RelationshipTypeSelect
          v-model="relationshipTypeCode"
          :source-kind="props.sourceKind ?? 'SUBJECT_KIND_DOCUMENT'"
          @selected="onTypeSelected"
        />

        <!-- The target list follows the chosen type's target kind. -->
        <SubjectPicker
          v-if="targetKind"
          v-model="targetSubjectId"
          :exclude-id="props.sourceId"
          :kind="targetKind"
        />

        <p v-else class="text-caption text-medium-emphasis mb-4">{{ t('link.chooseTypeFirst') }}</p>

        <v-text-field
          v-model="roleDetail"
          :label="t('fields.relationship.role_detail')"
        />
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="open = false">{{ t('actions.common.cancel') }}</v-btn>

        <v-btn
          color="primary"
          :disabled="!targetSubjectId || !relationshipTypeCode"
          :loading="props.busy"
          variant="flat"
          @click="submit"
        >
          {{ t('actions.document.link') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
