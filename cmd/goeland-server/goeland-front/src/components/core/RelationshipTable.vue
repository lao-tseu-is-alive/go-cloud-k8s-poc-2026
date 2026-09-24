<script setup lang="ts">
  import type { SubjectRelationship } from '@/api/types'
  import { ref } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { formatDate, formatDateTime } from '@/utils/formatters'
  import EndRelationshipDialog from './EndRelationshipDialog.vue'

  defineProps<{
    relationships?: SubjectRelationship[]
    // when true, "end" and "unlink" actions are shown (guarded by the parent)
    canUnlink?: boolean
  }>()
  const emit = defineEmits<{ unlink: [rel: SubjectRelationship], ended: [rel: SubjectRelationship] }>()
  const { t } = useI18n()

  const endOpen = ref(false)
  const endTarget = ref<SubjectRelationship>()

  function askEnd (rel: SubjectRelationship) {
    endTarget.value = rel
    endOpen.value = true
  }

  // An end of validity in the future is a scheduled end; the edge is still open.
  function hasEnded (rel: SubjectRelationship): boolean {
    return !!rel.validTo && new Date(rel.validTo).getTime() <= Date.now()
  }
</script>

<template>
  <div>
    <p v-if="!relationships || relationships.length === 0" class="text-medium-emphasis">
      {{ t('messages.common.noData') }}
    </p>

    <v-table v-else density="comfortable">
      <thead>
        <tr>
          <th scope="col">{{ t('fields.relationship.source') }}</th>
          <th scope="col">{{ t('fields.relationship.type') }}</th>
          <th scope="col">{{ t('fields.relationship.target') }}</th>
          <th scope="col">{{ t('fields.relationship.role_detail') }}</th>
          <th scope="col">{{ t('fields.relationship.validity') }}</th>
          <th scope="col">{{ t('fields.relationship.created_at') }}</th>
          <th v-if="canUnlink" scope="col" />
        </tr>
      </thead>

      <tbody>
        <tr v-for="rel in relationships" :key="rel.id" :class="{ 'text-disabled': rel.deletedAt || hasEnded(rel) }">
          <td>{{ rel.source?.displayLabel }}</td>

          <td>
            <!-- Business label for display; the code is what travels to the API. -->
            <v-chip label size="small">{{ rel.relationshipType?.label ?? rel.relationshipType?.code }}</v-chip>
          </td>

          <td>{{ rel.target?.displayLabel }}</td>
          <td>{{ rel.roleDetail || '—' }}</td>

          <td class="text-caption">
            <span v-if="rel.validFrom">{{ formatDate(rel.validFrom) }}</span>
            <span v-if="rel.validFrom || rel.validTo"> → </span>
            <span v-if="rel.validTo">{{ formatDate(rel.validTo) }}</span>

            <v-chip
              v-if="rel.validTo"
              class="ml-1"
              :color="hasEnded(rel) ? 'grey' : 'warning'"
              label
              size="x-small"
            >
              {{ hasEnded(rel) ? t('fields.relationship.ended') : t('fields.relationship.ending') }}
            </v-chip>

            <span v-if="!rel.validFrom && !rel.validTo">—</span>
          </td>

          <td class="text-caption">{{ formatDateTime(rel.createdAt) }}</td>

          <td v-if="canUnlink" class="text-right text-no-wrap">
            <v-btn
              v-if="!rel.deletedAt && !rel.validTo"
              :aria-label="t('actions.relationship.end')"
              color="warning"
              icon="mdi-calendar-end"
              size="small"
              :title="t('actions.relationship.end')"
              variant="text"
              @click="askEnd(rel)"
            />

            <v-btn
              v-if="!rel.deletedAt"
              :aria-label="t('actions.relationship.unlink')"
              color="error"
              icon="mdi-link-off"
              size="small"
              :title="t('actions.relationship.unlink')"
              variant="text"
              @click="emit('unlink', rel)"
            />
          </td>
        </tr>
      </tbody>
    </v-table>

    <EndRelationshipDialog v-model="endOpen" :relationship="endTarget" @ended="emit('ended', $event)" />
  </div>
</template>
