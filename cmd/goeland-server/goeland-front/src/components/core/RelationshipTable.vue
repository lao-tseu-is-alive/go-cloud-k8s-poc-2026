<script setup lang="ts">
  import type { SubjectRelationship } from '@/api/types'
  import { useI18n } from 'vue-i18n'
  import { formatDateTime } from '@/utils/formatters'

  defineProps<{
    relationships?: SubjectRelationship[]
    // when true, an "unlink" action column is shown (guarded by the parent)
    canUnlink?: boolean
  }>()
  const emit = defineEmits<{ unlink: [rel: SubjectRelationship] }>()
  const { t } = useI18n()
</script>

<template>
  <div>
    <p v-if="!relationships || relationships.length === 0" class="text-medium-emphasis">
      {{ t('messages.common.noData') }}
    </p>

    <v-table v-else density="comfortable">
      <thead>
        <tr>
          <th>{{ t('fields.relationship.source') }}</th>
          <th>{{ t('fields.relationship.type') }}</th>
          <th>{{ t('fields.relationship.target') }}</th>
          <th>{{ t('fields.relationship.role_detail') }}</th>
          <th>{{ t('fields.relationship.created_at') }}</th>
          <th v-if="canUnlink" />
        </tr>
      </thead>

      <tbody>
        <tr v-for="rel in relationships" :key="rel.id" :class="{ 'text-disabled': rel.deletedAt }">
          <td>{{ rel.source?.displayLabel }}</td>

          <td>
            <!-- Business label for display; the code is what travels to the API. -->
            <v-chip label size="small">{{ rel.relationshipType?.label ?? rel.relationshipType?.code }}</v-chip>
          </td>

          <td>{{ rel.target?.displayLabel }}</td>
          <td>{{ rel.roleDetail || '—' }}</td>
          <td class="text-caption">{{ formatDateTime(rel.createdAt) }}</td>

          <td v-if="canUnlink" class="text-right">
            <v-btn
              v-if="!rel.deletedAt"
              color="error"
              icon="mdi-link-off"
              size="small"
              variant="text"
              @click="emit('unlink', rel)"
            />
          </td>
        </tr>
      </tbody>
    </v-table>
  </div>
</template>
