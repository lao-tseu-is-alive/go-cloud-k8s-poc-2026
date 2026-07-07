<script setup lang="ts">
  import type { SubjectKind } from '@/api/types'
  import { ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import RelationshipTypeSelect from './RelationshipTypeSelect.vue'

  // Collects the inputs for a typed link. It emits a `submit` with the payload;
  // the parent owns the actual API call so this stays a pure input component.
  const props = defineProps<{ sourceKind?: SubjectKind, busy?: boolean }>()
  const emit = defineEmits<{ submit: [payload: { targetSubjectId: string, relationshipTypeCode: string, roleDetail: string }] }>()
  const open = defineModel<boolean>({ required: true })

  const { t } = useI18n()
  const targetSubjectId = ref('')
  const relationshipTypeCode = ref<string | undefined>(undefined)
  const roleDetail = ref('')

  watch(open, isOpen => {
    if (isOpen) {
      targetSubjectId.value = ''
      relationshipTypeCode.value = undefined
      roleDetail.value = ''
    }
  })

  function submit () {
    if (!targetSubjectId.value || !relationshipTypeCode.value) return
    emit('submit', {
      targetSubjectId: targetSubjectId.value.trim(),
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
        />

        <v-text-field
          v-model="targetSubjectId"
          :label="t('link.targetId')"
          placeholder="00000000-0000-0000-0000-000000000000"
        />

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
