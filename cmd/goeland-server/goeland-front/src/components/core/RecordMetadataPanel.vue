<script setup lang="ts">
  import type { RecordMetadata } from '@/api/types'
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { formatDateTime } from '@/utils/formatters'

  const props = defineProps<{ metadata?: RecordMetadata }>()
  const { t } = useI18n()

  interface Row { key: string, value: string }
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
          { key: 'created_by', value: fmt(m.createdBy) },
          { key: 'updated_at', value: formatDateTime(m.updatedAt) },
          { key: 'updated_by', value: fmt(m.updatedBy) },
          { key: 'deleted_at', value: m.deletedAt ? formatDateTime(m.deletedAt) : '—' },
          { key: 'deleted_by', value: fmt(m.deletedBy) },
        ],
      },
      {
        id: 'ownership',
        rows: [
          { key: 'owner_user_id', value: fmt(m.ownerUserId) },
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
          { key: 'locked_by', value: fmt(m.lockedBy) },
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
            <td>{{ row.value }}</td>
          </tr>
        </tbody>
      </v-table>
    </div>
  </div>
</template>
