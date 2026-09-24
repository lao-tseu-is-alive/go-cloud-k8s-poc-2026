<script setup lang="ts">
  import type { RecordMetadata } from '@/api/types'
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { formatDateTime } from '@/utils/formatters'
  import UserLabel from './UserLabel.vue'

  const props = defineProps<{ metadata?: RecordMetadata }>()
  const { t } = useI18n()

  // A row holds either a formatted value or an operator id shown by name.
  interface Row { key: string, value?: string, userId?: string }
  interface Group { id: string, rows: Row[] }

  function fmt (v: unknown): string {
    if (([undefined, null, ''] as unknown[]).includes(v)) return '—'
    if (typeof v === 'boolean') return v ? '✓' : '—'
    return String(v)
  }

  const groups = computed<Group[]>(() => {
    const m = props.metadata ?? {}
    return [
      {
        id: 'lifecycle',
        rows: [
          { key: 'created_at', value: formatDateTime(m.createdAt) },
          { key: 'created_by', userId: m.createdBy },
          { key: 'updated_at', value: formatDateTime(m.updatedAt) },
          { key: 'updated_by', userId: m.updatedBy },
          { key: 'deleted_at', value: m.deletedAt ? formatDateTime(m.deletedAt) : '—' },
          { key: 'deleted_by', userId: m.deletedBy },
        ],
      },
      {
        id: 'ownership',
        rows: [
          { key: 'owner_user_id', userId: m.ownerUserId },
          { key: 'owner_org_id', value: fmt(m.ownerOrgId) },
          { key: 'confidentiality_level', value: fmt(m.confidentialityLevel) },
        ],
      },
      {
        id: 'locking',
        rows: [
          { key: 'version', value: fmt(m.version) },
          { key: 'is_locked', value: fmt(m.isLocked) },
          { key: 'locked_at', value: m.lockedAt ? formatDateTime(m.lockedAt) : '—' },
          { key: 'locked_by', userId: m.lockedBy },
        ],
      },
      {
        id: 'retention',
        rows: [
          { key: 'retention_until', value: m.retentionUntil ? formatDateTime(m.retentionUntil) : '—' },
          { key: 'sort_final', value: fmt(m.sortFinal) },
        ],
      },
    ]
  })
</script>

<template>
  <div>
    <div v-for="group in groups" :key="group.id" class="mb-4">
      <div class="text-overline text-medium-emphasis">{{ t(`sections.recordMetadata.${group.id}`) }}</div>

      <v-table density="compact">
        <tbody>
          <tr v-for="row in group.rows" :key="row.key">
            <td class="text-medium-emphasis" style="width: 45%">{{ t(`fields.recordMetadata.${row.key}`) }}</td>

            <td>
              <UserLabel v-if="row.userId" :id="row.userId" />
              <span v-else>{{ row.value ?? '—' }}</span>
            </td>
          </tr>
        </tbody>
      </v-table>
    </div>
  </div>
</template>
