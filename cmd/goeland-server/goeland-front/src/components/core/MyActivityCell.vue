<script setup lang="ts">
  import type { SubjectActivity } from '@/api/types'
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { lastActivity } from '@/utils/activity'
  import { formatDateTime } from '@/utils/formatters'

  // The caller's latest activity on a row of « Mon tableau de bord » (GLD-088):
  // « Modifié » or « Consulté » and when.
  const props = defineProps<{ activity?: SubjectActivity }>()
  const { t } = useI18n()
  const last = computed(() => lastActivity(props.activity))
</script>

<template>
  <td class="text-caption text-no-wrap">
    <template v-if="last">
      <v-icon class="mr-1" :icon="last.kind === 'changed' ? 'mdi-pencil' : 'mdi-eye-outline'" size="x-small" />
      {{ t(`dashboard.activity.${last.kind}`) }} {{ formatDateTime(last.at) }}
    </template>

    <template v-else>—</template>
  </td>
</template>
